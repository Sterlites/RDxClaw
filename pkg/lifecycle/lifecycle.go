//go:build !windows
// +build !windows

// Package lifecycle provides zero-downtime binary upgrade support for RDxClaw.
//
// On Linux, it uses Cloudflare's tableflip library to perform graceful binary
// handoff: the old process enters drain mode, completes in-flight work, passes
// its listener file descriptors to the new binary, and exits cleanly.
//
// The Windows build uses a no-op stub (lifecycle_windows.go).
package lifecycle

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cloudflare/tableflip"
)

// DefaultDrainTimeout is how long the old process waits for in-flight requests
// before force-exiting. Set to 35 minutes because LLM calls can take up to 30 min.
const DefaultDrainTimeout = 35 * time.Minute

// DrainHook is a function called when the process begins draining.
// It should be non-blocking or complete quickly.
type DrainHook func()

// Manager orchestrates the process lifecycle for zero-downtime upgrades.
type Manager struct {
	upgrader     *tableflip.Upgrader
	draining     atomic.Bool
	drainTimeout time.Duration

	drainHooksMu sync.Mutex
	drainHooks   []DrainHook

	// done is closed when the upgrade exit sequence is complete
	done chan struct{}
}

// NewManager creates a new lifecycle manager.
// pidFile is optional — if non-empty, the PID file is written for systemd tracking.
func NewManager(pidFile string) (*Manager, error) {
	upg, err := tableflip.New(tableflip.Options{
		PIDFile: pidFile,
	})
	if err != nil {
		return nil, fmt.Errorf("lifecycle: failed to create upgrader: %w", err)
	}

	m := &Manager{
		upgrader:     upg,
		drainTimeout: DefaultDrainTimeout,
		done:         make(chan struct{}),
	}

	// Listen for SIGUSR2 to trigger upgrade
	go m.handleSignals()

	return m, nil
}

// SetDrainTimeout overrides the default drain timeout.
func (m *Manager) SetDrainTimeout(d time.Duration) {
	m.drainTimeout = d
}

// IsDraining returns true if the process is in drain mode (preparing to exit).
func (m *Manager) IsDraining() bool {
	return m.draining.Load()
}

// RegisterDrainHook adds a callback that will be invoked when drain begins.
// Hooks are called in registration order. Each hook should complete quickly
// (e.g., persist state, stop accepting new work). Long-running cleanup should
// use the drain timeout instead.
func (m *Manager) RegisterDrainHook(fn DrainHook) {
	m.drainHooksMu.Lock()
	defer m.drainHooksMu.Unlock()
	m.drainHooks = append(m.drainHooks, fn)
}

// Listen returns a net.Listener that is inherited across process upgrades.
// On upgrade, the new process receives the same listener FD, so the port
// is never released.
func (m *Manager) Listen(network, addr string) (net.Listener, error) {
	return m.upgrader.Listen(network, addr)
}

// Ready signals to tableflip that the new process is initialized and ready
// to accept traffic. This should be called after all services are started.
func (m *Manager) Ready() error {
	return m.upgrader.Ready()
}

// Done returns a channel that is closed when the process should exit.
func (m *Manager) Done() <-chan struct{} {
	return m.done
}

// UpgraderExit returns the tableflip exit channel.
// When this fires, the old process should begin its shutdown sequence.
func (m *Manager) UpgraderExit() <-chan struct{} {
	return m.upgrader.Exit()
}

// Stop cleans up the upgrader resources.
func (m *Manager) Stop() {
	m.upgrader.Stop()
}

// handleSignals listens for SIGUSR2 (upgrade) and SIGINT/SIGTERM (shutdown).
func (m *Manager) handleSignals() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGUSR2, syscall.SIGINT, syscall.SIGTERM)

	for s := range sig {
		switch s {
		case syscall.SIGUSR2:
			// Upgrade triggered — tableflip handles the fork internally
			// when Upgrade() is called. We just need to handle the exit.
			fmt.Println("[lifecycle] SIGUSR2 received — initiating graceful upgrade")

			// The upgrader already handles SIGUSR2 internally via tableflip,
			// so we just need to wait for the Exit() signal and run drain.
			// The actual fork happens inside tableflip when it receives SIGUSR2.

		case syscall.SIGINT, syscall.SIGTERM:
			fmt.Println("[lifecycle] Shutdown signal received — draining")
			m.executeDrain()
			close(m.done)
			return
		}
	}
}

// WaitForUpgradeAndDrain blocks until an upgrade is triggered, then executes
// the drain sequence. This should be called in a goroutine from the main
// server setup.
func (m *Manager) WaitForUpgradeAndDrain() {
	// Wait for tableflip to signal that the new process is ready
	<-m.upgrader.Exit()

	fmt.Println("[lifecycle] New process is ready — beginning drain of old process")
	m.executeDrain()
	close(m.done)
}

// executeDrain runs all drain hooks and sets the draining flag.
func (m *Manager) executeDrain() {
	m.draining.Store(true)

	m.drainHooksMu.Lock()
	hooks := make([]DrainHook, len(m.drainHooks))
	copy(hooks, m.drainHooks)
	m.drainHooksMu.Unlock()

	for i, hook := range hooks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[lifecycle] Drain hook %d panicked: %v\n", i, r)
				}
			}()
			hook()
		}()
	}

	fmt.Println("[lifecycle] All drain hooks executed")
}

// WaitForDrain waits for in-flight work to complete (via the provided WaitFunc)
// up to the drain timeout.
func (m *Manager) WaitForDrain(waitFn func()) {
	done := make(chan struct{})
	go func() {
		waitFn()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("[lifecycle] Drain complete — all in-flight work finished")
	case <-time.After(m.drainTimeout):
		fmt.Printf("[lifecycle] Drain timeout (%v) exceeded — forcing exit\n", m.drainTimeout)
	}
}

// GracefulContext returns a context that is cancelled when drain begins.
// Use this as the parent context for new requests so they are automatically
// cancelled if drain takes too long.
func (m *Manager) GracefulContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	go func() {
		select {
		case <-m.done:
			cancel()
		case <-parent.Done():
		}
	}()

	return ctx, cancel
}
