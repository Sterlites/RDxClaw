package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Persistence handles the atomic saving and loading of swarm tasks.
type Persistence struct {
	mu        sync.Mutex
	tasksFile string
}

func NewPersistence(workspace string) *Persistence {
	tasksDir := filepath.Join(workspace, "swarm")
	if err := os.MkdirAll(tasksDir, 0700); err != nil {
		// Just log error informally as we don't have a logger here yet
		fmt.Printf("[ERROR] swarm: failed to create tasks directory: %v\n", err)
	}
	return &Persistence{
		tasksFile: filepath.Join(tasksDir, "tasks.json"),
	}
}

func (p *Persistence) SaveTasks(tasks map[string]*SubagentTask) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Convert tasks map to a slice for cleaner JSON
	var taskList []*SubagentTask
	for _, t := range tasks {
		taskList = append(taskList, t)
	}

	data, err := json.MarshalIndent(taskList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal swarm tasks: %w", err)
	}

	// Atomic save: write to temp file then rename
	tempFile := p.tasksFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp tasks file: %w", err)
	}

	if err := os.Rename(tempFile, p.tasksFile); err != nil {
		_ = os.Remove(tempFile) // Cleanup
		return fmt.Errorf("failed to rename temp tasks file: %w", err)
	}

	return nil
}

func (p *Persistence) LoadTasks() (map[string]*SubagentTask, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(p.tasksFile); os.IsNotExist(err) {
		return make(map[string]*SubagentTask), nil
	}

	data, err := os.ReadFile(p.tasksFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read swarm tasks file: %w", err)
	}

	var taskList []*SubagentTask
	if err := json.Unmarshal(data, &taskList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal swarm tasks: %w", err)
	}

	tasks := make(map[string]*SubagentTask)
	for _, t := range taskList {
		// Note: we can't recover the cancel function, but we can mark them as "interrupted" or "lapsed"
		if t.Status == "running" {
			t.Status = "lapsed"
		}
		tasks[t.ID] = t
	}

	return tasks, nil
}
