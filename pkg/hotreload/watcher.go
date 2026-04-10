// Package hotreload provides file-system watching for live configuration and
// content reload without requiring a process restart.
//
// It uses fsnotify to watch workspace directories (skills, prompts, memory,
// config) and triggers callbacks when files change. Includes debouncing to
// avoid rapid-fire reloads during multi-file saves.
package hotreload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounce is how long to wait after the last file change event
// before triggering the callback. This prevents rapid-fire reloads when
// editors write files in multiple steps (write temp → rename).
const DefaultDebounce = 500 * time.Millisecond

// WatchRule defines a file-watching rule: a directory or file pattern and
// the callback to invoke when a change is detected.
type WatchRule struct {
	// Path is the directory or file to watch.
	Path string

	// Recursive controls whether subdirectories are watched.
	Recursive bool

	// Extensions filters events to only these file extensions (e.g., ".md", ".json").
	// If empty, all files trigger the callback.
	Extensions []string

	// Callback is invoked when a matching file change is detected.
	// The argument is the path of the changed file.
	Callback func(changedPath string)

	// Label is a human-readable name for logging.
	Label string
}

// Watcher watches filesystem paths and triggers callbacks on changes.
type Watcher struct {
	rules    []WatchRule
	debounce time.Duration
	watcher  *fsnotify.Watcher
	stopCh   chan struct{}
	stopped  sync.Once
}

// NewWatcher creates a new hot-reload watcher with the given debounce duration.
func NewWatcher(debounce time.Duration) *Watcher {
	if debounce <= 0 {
		debounce = DefaultDebounce
	}
	return &Watcher{
		debounce: debounce,
		stopCh:   make(chan struct{}),
	}
}

// AddRule adds a watch rule. Must be called before Start().
func (w *Watcher) AddRule(rule WatchRule) {
	w.rules = append(w.rules, rule)
}

// Start begins watching all registered paths. This method blocks; call it in a goroutine.
func (w *Watcher) Start() error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("hotreload: failed to create watcher: %w", err)
	}
	w.watcher = fsw

	// Register all paths
	for _, rule := range w.rules {
		if err := w.addPath(rule.Path, rule.Recursive); err != nil {
			fmt.Printf("[hotreload] Warning: failed to watch %s (%s): %v\n", rule.Path, rule.Label, err)
			continue
		}
		fmt.Printf("[hotreload] Watching %s (%s)\n", rule.Path, rule.Label)
	}

	// Event loop with debouncing
	w.eventLoop()
	return nil
}

// Stop halts the watcher.
func (w *Watcher) Stop() {
	w.stopped.Do(func() {
		close(w.stopCh)
		if w.watcher != nil {
			_ = w.watcher.Close()
		}
	})
}

// addPath adds a path (and optionally its subdirectories) to the watcher.
func (w *Watcher) addPath(path string, recursive bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		// Watch the parent directory and filter by filename
		return w.watcher.Add(filepath.Dir(path))
	}

	if err := w.watcher.Add(path); err != nil {
		return err
	}

	if recursive {
		return filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err != nil || !fi.IsDir() {
				return nil
			}
			return w.watcher.Add(p)
		})
	}

	return nil
}

// eventLoop processes fsnotify events with debouncing.
func (w *Watcher) eventLoop() {
	// Debounce timers per rule index
	timers := make(map[int]*time.Timer)
	var mu sync.Mutex

	for {
		select {
		case <-w.stopCh:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only care about writes and creates (not chmod, remove)
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			// Find matching rules
			for i, rule := range w.rules {
				if !w.matchesRule(event.Name, rule) {
					continue
				}

				// Debounce: reset timer for this rule
				mu.Lock()
				ruleIdx := i
				changedPath := event.Name
				if t, exists := timers[ruleIdx]; exists {
					t.Stop()
				}
				timers[ruleIdx] = time.AfterFunc(w.debounce, func() {
					fmt.Printf("[hotreload] Reloading %s (triggered by %s)\n", rule.Label, filepath.Base(changedPath))
					func() {
						defer func() {
							if r := recover(); r != nil {
								fmt.Printf("[hotreload] Callback panic for %s: %v\n", rule.Label, r)
							}
						}()
						rule.Callback(changedPath)
					}()
				})
				mu.Unlock()
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("[hotreload] Watcher error: %v\n", err)
		}
	}
}

// matchesRule checks if a file event matches a watch rule.
func (w *Watcher) matchesRule(path string, rule WatchRule) bool {
	// Check path prefix
	absRule, err := filepath.Abs(rule.Path)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// For file rules, check exact match or parent directory
	info, _ := os.Stat(rule.Path)
	if info != nil && !info.IsDir() {
		return absPath == absRule
	}

	// For directory rules, check if the file is under the watched directory
	if !strings.HasPrefix(absPath, absRule) {
		return false
	}

	// Check extension filter
	if len(rule.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(path))
		matched := false
		for _, allowed := range rule.Extensions {
			if ext == strings.ToLower(allowed) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}
