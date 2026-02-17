package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillManifest defines the structure of a skill's manifest.json file.
// This is the "App Package" specification — it bundles prompts, scripts,
// cron schedules, webhook subscriptions, and environment requirements
// into a single installable unit.
type SkillManifest struct {
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	Description  string        `json:"description"`
	Author       string        `json:"author,omitempty"`
	License      string        `json:"license,omitempty"`
	EnvVars      []EnvVarSpec  `json:"env_vars,omitempty"`
	Scripts      []ScriptSpec  `json:"scripts,omitempty"`
	Cron         []CronSpec    `json:"cron,omitempty"`
	Webhooks     []WebhookSpec `json:"webhooks,omitempty"`
	Assets       []string      `json:"assets,omitempty"`
	Dependencies []string      `json:"dependencies,omitempty"`
}

// EnvVarSpec defines an environment variable required by the skill.
// During installation, the user is prompted for required vars that
// don't have a default value set.
type EnvVarSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// ScriptSpec defines an executable script bundled with the skill.
type ScriptSpec struct {
	Path        string `json:"path"`
	Runtime     string `json:"runtime"` // python, node, go, shell
	Description string `json:"description,omitempty"`
	Entrypoint  bool   `json:"entrypoint,omitempty"` // true if this is the main script
}

// CronSpec defines a scheduled job that the skill auto-registers on install.
type CronSpec struct {
	Name    string `json:"name"`
	Expr    string `json:"expr"`    // cron expression (e.g. "0 * * * *")
	Task    string `json:"task"`    // message/prompt to execute
	Enabled bool   `json:"enabled"` // whether to enable on install
}

// WebhookSpec defines a webhook endpoint the skill subscribes to.
type WebhookSpec struct {
	Path        string `json:"path"`        // URL path suffix (e.g. "/shopify")
	Description string `json:"description"` // human-readable description
}

// LoadManifest reads and parses a manifest.json from the given skill directory.
// Returns nil, nil if no manifest.json exists (backward-compatible with SKILL.md-only skills).
func LoadManifest(skillDir string) (*SkillManifest, error) {
	manifestPath := filepath.Join(skillDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No manifest — legacy skill
		}
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest SkillManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest.json: %w", err)
	}

	return &manifest, nil
}

// SaveManifest writes a manifest.json to the given skill directory.
func SaveManifest(skillDir string, manifest *SkillManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(skillDir, "manifest.json")
	return os.WriteFile(manifestPath, data, 0644)
}

// Validate checks that the manifest has required fields and valid values.
func (m *SkillManifest) Validate() error {
	var errs []string

	if m.Name == "" {
		errs = append(errs, "name is required")
	} else if !namePattern.MatchString(m.Name) {
		errs = append(errs, fmt.Sprintf("name %q must match pattern [a-zA-Z0-9]+(-[a-zA-Z0-9]+)*", m.Name))
	}
	if len(m.Name) > MaxNameLength {
		errs = append(errs, fmt.Sprintf("name must be at most %d characters", MaxNameLength))
	}

	if m.Version == "" {
		errs = append(errs, "version is required")
	}

	if m.Description == "" {
		errs = append(errs, "description is required")
	}

	validRuntimes := map[string]bool{"python": true, "node": true, "go": true, "shell": true}
	for i, s := range m.Scripts {
		if s.Path == "" {
			errs = append(errs, fmt.Sprintf("scripts[%d].path is required", i))
		}
		if s.Runtime == "" {
			errs = append(errs, fmt.Sprintf("scripts[%d].runtime is required", i))
		} else if !validRuntimes[s.Runtime] {
			errs = append(errs, fmt.Sprintf("scripts[%d].runtime %q must be one of: python, node, go, shell", i, s.Runtime))
		}
	}

	for i, c := range m.Cron {
		if c.Name == "" {
			errs = append(errs, fmt.Sprintf("cron[%d].name is required", i))
		}
		if c.Expr == "" {
			errs = append(errs, fmt.Sprintf("cron[%d].expr is required", i))
		}
		if c.Task == "" {
			errs = append(errs, fmt.Sprintf("cron[%d].task is required", i))
		}
	}

	for i, w := range m.Webhooks {
		if w.Path == "" {
			errs = append(errs, fmt.Sprintf("webhooks[%d].path is required", i))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// HasScripts returns true if the manifest defines any executable scripts.
func (m *SkillManifest) HasScripts() bool {
	return len(m.Scripts) > 0
}

// HasCron returns true if the manifest defines any cron jobs.
func (m *SkillManifest) HasCron() bool {
	return len(m.Cron) > 0
}

// HasWebhooks returns true if the manifest defines any webhook subscriptions.
func (m *SkillManifest) HasWebhooks() bool {
	return len(m.Webhooks) > 0
}

// RequiredEnvVars returns the list of environment variables that are required
// and don't have a default value.
func (m *SkillManifest) RequiredEnvVars() []EnvVarSpec {
	var required []EnvVarSpec
	for _, v := range m.EnvVars {
		if v.Required && v.Default == "" {
			required = append(required, v)
		}
	}
	return required
}

// CapabilitiesSummary returns a human-readable summary of the skill's capabilities.
func (m *SkillManifest) CapabilitiesSummary() string {
	var parts []string
	if len(m.Scripts) > 0 {
		parts = append(parts, fmt.Sprintf("%d script(s)", len(m.Scripts)))
	}
	if len(m.Cron) > 0 {
		parts = append(parts, fmt.Sprintf("%d cron job(s)", len(m.Cron)))
	}
	if len(m.Webhooks) > 0 {
		parts = append(parts, fmt.Sprintf("%d webhook(s)", len(m.Webhooks)))
	}
	if len(m.EnvVars) > 0 {
		parts = append(parts, fmt.Sprintf("%d env var(s)", len(m.EnvVars)))
	}
	if len(parts) == 0 {
		return "prompt-only"
	}
	return strings.Join(parts, ", ")
}
