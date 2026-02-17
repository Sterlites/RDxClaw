package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name      string
		manifest  SkillManifest
		wantError bool
		errContains string
	}{
		{
			name: "valid minimal manifest",
			manifest: SkillManifest{
				Name:        "test-skill",
				Version:     "1.0.0",
				Description: "A test skill",
			},
			wantError: false,
		},
		{
			name: "valid full manifest",
			manifest: SkillManifest{
				Name:        "shopify-refund",
				Version:     "1.0.0",
				Description: "Shopify refund manager",
				Author:      "RDx",
				License:     "MIT",
				EnvVars: []EnvVarSpec{
					{Name: "SHOPIFY_API_KEY", Description: "Your Shopify API key", Required: true},
					{Name: "LOG_LEVEL", Description: "Log level", Required: false, Default: "info"},
				},
				Scripts: []ScriptSpec{
					{Path: "scripts/logic.py", Runtime: "python", Description: "Main logic", Entrypoint: true},
				},
				Cron: []CronSpec{
					{Name: "check-refunds", Expr: "0 * * * *", Task: "Check for new refund requests", Enabled: true},
				},
				Webhooks: []WebhookSpec{
					{Path: "/shopify", Description: "Shopify webhook receiver"},
				},
			},
			wantError: false,
		},
		{
			name:        "missing name",
			manifest:    SkillManifest{Version: "1.0.0", Description: "test"},
			wantError:   true,
			errContains: "name is required",
		},
		{
			name:        "missing version",
			manifest:    SkillManifest{Name: "test", Description: "test"},
			wantError:   true,
			errContains: "version is required",
		},
		{
			name:        "missing description",
			manifest:    SkillManifest{Name: "test", Version: "1.0.0"},
			wantError:   true,
			errContains: "description is required",
		},
		{
			name: "invalid script runtime",
			manifest: SkillManifest{
				Name: "test", Version: "1.0.0", Description: "test",
				Scripts: []ScriptSpec{{Path: "run.rb", Runtime: "ruby"}},
			},
			wantError:   true,
			errContains: "must be one of",
		},
		{
			name: "missing script path",
			manifest: SkillManifest{
				Name: "test", Version: "1.0.0", Description: "test",
				Scripts: []ScriptSpec{{Runtime: "python"}},
			},
			wantError:   true,
			errContains: "path is required",
		},
		{
			name: "missing cron expr",
			manifest: SkillManifest{
				Name: "test", Version: "1.0.0", Description: "test",
				Cron: []CronSpec{{Name: "job1", Task: "do thing"}},
			},
			wantError:   true,
			errContains: "expr is required",
		},
		{
			name: "invalid skill name",
			manifest: SkillManifest{
				Name: "invalid name!", Version: "1.0.0", Description: "test",
			},
			wantError:   true,
			errContains: "must match pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadManifest(t *testing.T) {
	t.Run("loads valid manifest", func(t *testing.T) {
		dir := t.TempDir()
		manifestJSON := `{
			"name": "twitter-manager",
			"version": "1.0.0",
			"description": "Manages Twitter posts",
			"author": "RDx",
			"env_vars": [
				{"name": "TWITTER_API_KEY", "description": "API Key", "required": true}
			],
			"scripts": [
				{"path": "scripts/post.py", "runtime": "python", "entrypoint": true}
			],
			"cron": [
				{"name": "hourly-post", "expr": "0 * * * *", "task": "Post scheduled tweet", "enabled": true}
			]
		}`
		err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestJSON), 0644)
		require.NoError(t, err)

		m, err := LoadManifest(dir)
		require.NoError(t, err)
		require.NotNil(t, m)

		assert.Equal(t, "twitter-manager", m.Name)
		assert.Equal(t, "1.0.0", m.Version)
		assert.Len(t, m.EnvVars, 1)
		assert.Len(t, m.Scripts, 1)
		assert.Len(t, m.Cron, 1)
		assert.True(t, m.Scripts[0].Entrypoint)
	})

	t.Run("returns nil for missing manifest", func(t *testing.T) {
		dir := t.TempDir()
		m, err := LoadManifest(dir)
		assert.NoError(t, err)
		assert.Nil(t, m)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{bad json}"), 0644)
		require.NoError(t, err)

		m, err := LoadManifest(dir)
		assert.Error(t, err)
		assert.Nil(t, m)
	})

	t.Run("returns error for invalid manifest", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"name":"","version":"1.0.0","description":"x"}`), 0644)
		require.NoError(t, err)

		m, err := LoadManifest(dir)
		assert.Error(t, err)
		assert.Nil(t, m)
	})
}

func TestSaveManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := &SkillManifest{
		Name:        "test-skill",
		Version:     "1.0.0",
		Description: "Test",
	}

	err := SaveManifest(dir, manifest)
	require.NoError(t, err)

	loaded, err := LoadManifest(dir)
	require.NoError(t, err)
	assert.Equal(t, manifest.Name, loaded.Name)
	assert.Equal(t, manifest.Version, loaded.Version)
}

func TestManifestHelpers(t *testing.T) {
	m := &SkillManifest{
		Name:    "test",
		Version: "1.0.0",
		Description: "test",
		EnvVars: []EnvVarSpec{
			{Name: "KEY1", Required: true},
			{Name: "KEY2", Required: false, Default: "val"},
			{Name: "KEY3", Required: true, Default: "val"},
		},
		Scripts:  []ScriptSpec{{Path: "run.py", Runtime: "python"}},
		Cron:     []CronSpec{{Name: "j", Expr: "* * * * *", Task: "t", Enabled: true}},
		Webhooks: []WebhookSpec{{Path: "/hook"}},
	}

	assert.True(t, m.HasScripts())
	assert.True(t, m.HasCron())
	assert.True(t, m.HasWebhooks())

	required := m.RequiredEnvVars()
	assert.Len(t, required, 1)
	assert.Equal(t, "KEY1", required[0].Name)

	summary := m.CapabilitiesSummary()
	assert.Contains(t, summary, "1 script(s)")
	assert.Contains(t, summary, "1 cron job(s)")
	assert.Contains(t, summary, "1 webhook(s)")
	assert.Contains(t, summary, "3 env var(s)")
}

func TestCapabilitiesSummaryPromptOnly(t *testing.T) {
	m := &SkillManifest{Name: "simple", Version: "1.0.0", Description: "simple"}
	assert.Equal(t, "prompt-only", m.CapabilitiesSummary())
}
