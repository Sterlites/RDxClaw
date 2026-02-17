package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

const (
	MaxNameLength        = 64
	MaxDescriptionLength = 1024
)

type SkillMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SkillInfo struct {
	Name         string         `json:"name"`
	Path         string         `json:"path"`
	Source       string         `json:"source"`
	Description  string         `json:"description"`
	Manifest     *SkillManifest `json:"manifest,omitempty"`
	Capabilities string         `json:"capabilities,omitempty"` // e.g. "2 script(s), 1 cron job(s)"
}

func (info SkillInfo) validate() error {
	var errs error
	if info.Name == "" {
		errs = errors.Join(errs, errors.New("name is required"))
	} else {
		if len(info.Name) > MaxNameLength {
			errs = errors.Join(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
		}
		if !namePattern.MatchString(info.Name) {
			errs = errors.Join(errs, errors.New("name must be alphanumeric with hyphens"))
		}
	}

	if info.Description == "" {
		errs = errors.Join(errs, errors.New("description is required"))
	} else if len(info.Description) > MaxDescriptionLength {
		errs = errors.Join(errs, fmt.Errorf("description exceeds %d character", MaxDescriptionLength))
	}
	return errs
}

type SkillsLoader struct {
	workspace       string
	workspaceSkills string // workspace skills (项目级别)
	globalSkills    string // 全局 skills (~/.rdxclaw/skills)
	builtinSkills   string // 内置 skills
}

func NewSkillsLoader(workspace string, globalSkills string, builtinSkills string) *SkillsLoader {
	return &SkillsLoader{
		workspace:       workspace,
		workspaceSkills: filepath.Join(workspace, "skills"),
		globalSkills:    globalSkills, // ~/.rdxclaw/skills
		builtinSkills:   builtinSkills,
	}
}

func (sl *SkillsLoader) ListSkills() []SkillInfo {
	skills := make([]SkillInfo, 0)

	if sl.workspaceSkills != "" {
		if dirs, err := os.ReadDir(sl.workspaceSkills); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					info := sl.loadSkillInfo(sl.workspaceSkills, dir.Name(), "workspace")
					if info != nil {
						skills = append(skills, *info)
					}
				}
			}
		}
	}

	// 全局 skills (~/.rdxclaw/skills) - 被 workspace skills 覆盖
	// Global skills (~/.rdxclaw/skills) — overridden by workspace skills
	if sl.globalSkills != "" {
		if dirs, err := os.ReadDir(sl.globalSkills); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					// Check if already overridden by workspace
					if sl.skillExists(skills, dir.Name(), "workspace") {
						continue
					}
					info := sl.loadSkillInfo(sl.globalSkills, dir.Name(), "global")
					if info != nil {
						skills = append(skills, *info)
					}
				}
			}
		}
	}

	// Builtin skills — overridden by workspace or global
	if sl.builtinSkills != "" {
		if dirs, err := os.ReadDir(sl.builtinSkills); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					if sl.skillExists(skills, dir.Name(), "workspace") || sl.skillExists(skills, dir.Name(), "global") {
						continue
					}
					info := sl.loadSkillInfo(sl.builtinSkills, dir.Name(), "builtin")
					if info != nil {
						skills = append(skills, *info)
					}
				}
			}
		}
	}

	return skills
}

// loadSkillInfo loads a skill from a directory. A skill is detected if it has
// either SKILL.md, manifest.json, or both. Manifest data takes priority.
func (sl *SkillsLoader) loadSkillInfo(baseDir, dirName, source string) *SkillInfo {
	skillDir := filepath.Join(baseDir, dirName)
	skillFile := filepath.Join(skillDir, "SKILL.md")
	_, hasSkillMD := os.Stat(skillFile), true
	if _, err := os.Stat(skillFile); err != nil {
		hasSkillMD = false
	}

	// Try loading manifest.json
	manifest, _ := LoadManifest(skillDir)

	// Skill must have at least SKILL.md or manifest.json
	if !hasSkillMD && manifest == nil {
		return nil
	}

	path := skillFile
	if !hasSkillMD && manifest != nil {
		path = filepath.Join(skillDir, "manifest.json")
	}

	info := SkillInfo{
		Name:     dirName,
		Path:     path,
		Source:   source,
		Manifest: manifest,
	}

	// Manifest data takes priority over SKILL.md frontmatter
	if manifest != nil {
		info.Name = manifest.Name
		info.Description = manifest.Description
		info.Capabilities = manifest.CapabilitiesSummary()
	} else if hasSkillMD {
		metadata := sl.getSkillMetadata(skillFile)
		if metadata != nil {
			info.Description = metadata.Description
			if metadata.Name != "" {
				info.Name = metadata.Name
			}
		}
		info.Capabilities = "prompt-only"
	}

	if err := info.validate(); err != nil {
		slog.Warn("invalid skill", "name", info.Name, "source", source, "error", err)
		return nil
	}

	return &info
}

// skillExists checks if a skill with the given directory name and source already exists.
func (sl *SkillsLoader) skillExists(skills []SkillInfo, dirName, source string) bool {
	for _, s := range skills {
		if s.Name == dirName && s.Source == source {
			return true
		}
	}
	return false
}

// GetSkillManifest returns the manifest for a named skill, or nil if not found.
func (sl *SkillsLoader) GetSkillManifest(name string) *SkillManifest {
	for _, s := range sl.ListSkills() {
		if s.Name == name && s.Manifest != nil {
			return s.Manifest
		}
	}
	return nil
}

func (sl *SkillsLoader) LoadSkill(name string) (string, bool) {
	// 1. 优先从 workspace skills 加载（项目级别）
	if sl.workspaceSkills != "" {
		skillFile := filepath.Join(sl.workspaceSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	// 2. 其次从全局 skills 加载 (~/.rdxclaw/skills)
	if sl.globalSkills != "" {
		skillFile := filepath.Join(sl.globalSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	// 3. 最后从内置 skills 加载
	if sl.builtinSkills != "" {
		skillFile := filepath.Join(sl.builtinSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	return "", false
}

func (sl *SkillsLoader) LoadSkillsForContext(skillNames []string) string {
	if len(skillNames) == 0 {
		return ""
	}

	var parts []string
	for _, name := range skillNames {
		content, ok := sl.LoadSkill(name)
		if ok {
			parts = append(parts, fmt.Sprintf("### Skill: %s\n\n%s", name, content))
		}
	}

	return strings.Join(parts, "\n\n---\n\n")
}

func (sl *SkillsLoader) BuildSkillsSummary() string {
	allSkills := sl.ListSkills()
	if len(allSkills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "<skills>")
	for _, s := range allSkills {
		escapedName := escapeXML(s.Name)
		escapedDesc := escapeXML(s.Description)
		escapedPath := escapeXML(s.Path)

		lines = append(lines, "  <skill>")
		lines = append(lines, fmt.Sprintf("    <name>%s</name>", escapedName))
		lines = append(lines, fmt.Sprintf("    <description>%s</description>", escapedDesc))
		lines = append(lines, fmt.Sprintf("    <location>%s</location>", escapedPath))
		lines = append(lines, fmt.Sprintf("    <source>%s</source>", s.Source))
		if s.Capabilities != "" {
			lines = append(lines, fmt.Sprintf("    <capabilities>%s</capabilities>", escapeXML(s.Capabilities)))
		}
		lines = append(lines, "  </skill>")
	}
	lines = append(lines, "</skills>")

	return strings.Join(lines, "\n")
}

func (sl *SkillsLoader) getSkillMetadata(skillPath string) *SkillMetadata {
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return nil
	}

	frontmatter := sl.extractFrontmatter(string(content))
	if frontmatter == "" {
		return &SkillMetadata{
			Name: filepath.Base(filepath.Dir(skillPath)),
		}
	}

	// Try JSON first (for backward compatibility)
	var jsonMeta struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(frontmatter), &jsonMeta); err == nil {
		return &SkillMetadata{
			Name:        jsonMeta.Name,
			Description: jsonMeta.Description,
		}
	}

	// Fall back to simple YAML parsing
	yamlMeta := sl.parseSimpleYAML(frontmatter)
	return &SkillMetadata{
		Name:        yamlMeta["name"],
		Description: yamlMeta["description"],
	}
}

// parseSimpleYAML parses simple key: value YAML format
// Example: name: github\n description: "..."
func (sl *SkillsLoader) parseSimpleYAML(content string) map[string]string {
	result := make(map[string]string)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, "\"'")
			result[key] = value
		}
	}

	return result
}

func (sl *SkillsLoader) extractFrontmatter(content string) string {
	// (?s) enables DOTALL mode so . matches newlines
	// Match first ---, capture everything until next --- on its own line
	re := regexp.MustCompile(`(?s)^---\n(.*)\n---`)
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func (sl *SkillsLoader) stripFrontmatter(content string) string {
	re := regexp.MustCompile(`^---\n.*?\n---\n`)
	return re.ReplaceAllString(content, "")
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
