//go:build windows
// +build windows

// Package lifecycle provides zero-downtime binary upgrade support for RDxClaw.
//
// On Windows, SIGUSR2 and socket inheritance are not available. This file
// provides a no-op stub that supports the same API surface so that the rest
// of the codebase compiles without build tags in caller code.
//
// Windows users must stop and restart the process manually.
package lifecycle

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultDrainTimeout is how long the old process waits for in-flight requests
// before force-exiting.
const DefaultDrainTimeout = 35 * time.Minute

// DrainHook is a function called when the process begins draining.
type DrainHook func()

// Manager is a no-op lifecycle manager on Windows.
type Manager struct {
	draining     atomic.Bool
	drainTimeout time.Duration

	drainHooksMu sync.Mutex
	drainHooks   []DrainHook

	done chan struct{}
}

// NewManager creates a no-op lifecycle manager on Windows.
// The pidFile parameter is ignored.
func NewManager(pidFile string) (*Manager, error) {
	m := &Manager{
		drainTimeout: DefaultDrainTimeout,
		done:         make(chan struct{}),
	}

	go m.handleSignals()
	return m, nil
}

// SetDrainTimeout overrides the default drain timeout.
func (m *Manager) SetDrainTimeout(d time.Duration) {
	m.drainTimeout = d
}

// IsDraining returns true if the process is in drain mode.
func (m *Manager) IsDraining() bool {
	return m.draining.Load()
}

// RegisterDrainHook adds a drain callback.
func (m *Manager) RegisterDrainHook(fn DrainHook) {
	m.drainHooksMu.Lock()
	defer m.drainHooksMu.Unlock()
	m.drainHooks = append(m.drainHooks, fn)
}

// Listen creates a standard net.Listener (no inheritance on Windows).
func (m *Manager) Listen(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}

// Ready is a no-op on Windows (no upgrade coordination).
func (m *Manager) Ready() error {
	return nil
}

// Done returns a channel closed when the process should exit.
func (m *Manager) Done() <-chan struct{} {
	return m.done
}

// UpgraderExit returns a channel that never fires on Windows (no upgrade support).
// On Windows, shutdown is only triggered by SIGINT/Ctrl+C.
func (m *Manager) UpgraderExit() <-chan struct{} {
	// Return a channel that never closes — upgrade is not supported on Windows
	return make(chan struct{})
}

// Stop is a no-op on Windows.
func (m *Manager) Stop() {}

// handleSignals handles SIGINT on Windows.
func (m *Manager) handleSignals() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	<-sig
	fmt.Println("[lifecycle] Shutdown signal received — draining")
	m.executeDrain()
	close(m.done)
}

// WaitForUpgradeAndDrain blocks forever on Windows (no upgrade support).
// Shutdown is only triggered by SIGINT. This method exists for API compatibility.
func (m *Manager) WaitForUpgradeAndDrain() {
	// On Windows, we just wait for the done channel (triggered by SIGINT)
	<-m.done
}

// executeDrain runs all drain hooks.
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
}

// WaitForDrain waits for in-flight work to complete up to the drain timeout.
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
