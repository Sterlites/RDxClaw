package skills

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SkillInstaller struct {
	workspace string
}

type AvailableSkill struct {
	Name        string   `json:"name"`
	Repository  string   `json:"repository"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
}

type BuiltinSkill struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

// InstallResult contains information about what was installed.
type InstallResult struct {
	Name         string
	SkillDir     string
	Manifest     *SkillManifest
	FilesWritten int
}

func NewSkillInstaller(workspace string) *SkillInstaller {
	return &SkillInstaller{
		workspace: workspace,
	}
}

// InstallFromGitHub downloads a skill package from a GitHub repository.
// It first tries to download the repo as a zip archive (multi-file skill package).
// If that fails, it falls back to downloading just the SKILL.md file (legacy behavior).
func (si *SkillInstaller) InstallFromGitHub(ctx context.Context, repo string) (*InstallResult, error) {
	skillName := filepath.Base(repo)
	skillDir := filepath.Join(si.workspace, "skills", skillName)

	if _, err := os.Stat(skillDir); err == nil {
		return nil, fmt.Errorf("skill '%s' already exists", skillName)
	}

	// Try downloading as zip archive first (full package)
	result, err := si.downloadRepoZip(ctx, repo, skillDir)
	if err == nil {
		return result, nil
	}
	slog.Debug("zip download failed, trying SKILL.md fallback", "repo", repo, "error", err)

	// Fallback: download just SKILL.md (legacy single-file skill)
	return si.downloadSkillMD(ctx, repo, skillDir)
}

// InstallFromArchive installs a skill from a local zip or tar.gz archive file.
func (si *SkillInstaller) InstallFromArchive(archivePath string) (*InstallResult, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))

	// Determine the skill name from the archive filename
	baseName := strings.TrimSuffix(filepath.Base(archivePath), ext)
	if ext == ".gz" && strings.HasSuffix(baseName, ".tar") {
		baseName = strings.TrimSuffix(baseName, ".tar")
	}

	skillDir := filepath.Join(si.workspace, "skills", baseName)
	if _, err := os.Stat(skillDir); err == nil {
		return nil, fmt.Errorf("skill '%s' already exists", baseName)
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skill directory: %w", err)
	}

	var filesWritten int
	var extractErr error

	switch {
	case ext == ".zip":
		filesWritten, extractErr = extractZip(archivePath, skillDir)
	case ext == ".gz" || ext == ".tgz":
		filesWritten, extractErr = extractTarGz(archivePath, skillDir)
	default:
		os.RemoveAll(skillDir)
		return nil, fmt.Errorf("unsupported archive format: %s (supported: .zip, .tar.gz, .tgz)", ext)
	}

	if extractErr != nil {
		os.RemoveAll(skillDir)
		return nil, fmt.Errorf("failed to extract archive: %w", extractErr)
	}

	// Load and validate manifest if present
	manifest, _ := LoadManifest(skillDir)

	name := baseName
	if manifest != nil {
		name = manifest.Name
	}

	return &InstallResult{
		Name:         name,
		SkillDir:     skillDir,
		Manifest:     manifest,
		FilesWritten: filesWritten,
	}, nil
}

func (si *SkillInstaller) Uninstall(skillName string) error {
	skillDir := filepath.Join(si.workspace, "skills", skillName)

	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return fmt.Errorf("skill '%s' not found", skillName)
	}

	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("failed to remove skill: %w", err)
	}

	return nil
}

// GetSkillManifest returns the manifest for an installed skill, or nil if none exists.
func (si *SkillInstaller) GetSkillManifest(skillName string) (*SkillManifest, error) {
	skillDir := filepath.Join(si.workspace, "skills", skillName)
	return LoadManifest(skillDir)
}

func (si *SkillInstaller) ListAvailableSkills(ctx context.Context) ([]AvailableSkill, error) {
	url := "https://raw.githubusercontent.com/Sterlites/rdxclaw-skills/main/skills.json"

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skills list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch skills list: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var skills []AvailableSkill
	if err := json.Unmarshal(body, &skills); err != nil {
		return nil, fmt.Errorf("failed to parse skills list: %w", err)
	}

	return skills, nil
}

func (si *SkillInstaller) ListBuiltinSkills() []BuiltinSkill {
	builtinSkillsDir := filepath.Join(filepath.Dir(si.workspace), "rdxclaw", "skills")

	entries, err := os.ReadDir(builtinSkillsDir)
	if err != nil {
		return nil
	}

	var skills []BuiltinSkill
	for _, entry := range entries {
		if entry.IsDir() {
			skillName := entry.Name()
			skillDir := filepath.Join(builtinSkillsDir, skillName)
			skillFile := filepath.Join(skillDir, "SKILL.md")

			description := ""
			// Prefer manifest.json if present
			if manifest, _ := LoadManifest(skillDir); manifest != nil {
				description = manifest.Description
			} else if data, err := os.ReadFile(skillFile); err == nil {
				content := string(data)
				if idx := strings.Index(content, "\n"); idx > 0 {
					firstLine := content[:idx]
					if strings.Contains(firstLine, "description:") {
						descLine := strings.Index(content[idx:], "\n")
						if descLine > 0 {
							description = strings.TrimSpace(content[idx+descLine : idx+descLine])
						}
					}
				}
			}

			status := "✓"
			fmt.Printf("  %s  %s\n", status, entry.Name())
			if description != "" {
				fmt.Printf("    %s\n", description)
			}
		}
	}
	return skills
}

// --- Internal helpers ---

func (si *SkillInstaller) downloadRepoZip(ctx context.Context, repo, skillDir string) (*InstallResult, error) {
	url := fmt.Sprintf("https://github.com/%s/archive/refs/heads/main.zip", repo)

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repo zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch repo zip: HTTP %d", resp.StatusCode)
	}

	// Write zip to temp file
	tmpFile, err := os.CreateTemp("", "rdxclaw-skill-*.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return nil, fmt.Errorf("failed to download zip: %w", err)
	}
	tmpFile.Close()

	// Extract zip — GitHub zips have a top-level directory like "repo-main/"
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skill directory: %w", err)
	}

	filesWritten, err := extractZipStripRoot(tmpFile.Name(), skillDir)
	if err != nil {
		os.RemoveAll(skillDir)
		return nil, fmt.Errorf("failed to extract zip: %w", err)
	}

	manifest, _ := LoadManifest(skillDir)
	name := filepath.Base(repo)
	if manifest != nil {
		name = manifest.Name
	}

	return &InstallResult{
		Name:         name,
		SkillDir:     skillDir,
		Manifest:     manifest,
		FilesWritten: filesWritten,
	}, nil
}

func (si *SkillInstaller) downloadSkillMD(ctx context.Context, repo, skillDir string) (*InstallResult, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/SKILL.md", repo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch skill: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skill directory: %w", err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, body, 0644); err != nil {
		return nil, fmt.Errorf("failed to write skill file: %w", err)
	}

	return &InstallResult{
		Name:         filepath.Base(repo),
		SkillDir:     skillDir,
		FilesWritten: 1,
	}, nil
}

// extractZip extracts a zip archive to the destination directory.
func extractZip(zipPath, destDir string) (int, error) {
	return extractZipStripRoot(zipPath, destDir)
}

// extractZipStripRoot extracts a zip, stripping the top-level directory if all
// files share one common root (as GitHub repo zips do).
func extractZipStripRoot(zipPath, destDir string) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	// Detect common root prefix
	var commonRoot string
	for _, f := range r.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) >= 2 {
			if commonRoot == "" {
				commonRoot = parts[0] + "/"
			} else if !strings.HasPrefix(f.Name, commonRoot) {
				commonRoot = "" // No common root
				break
			}
		}
	}

	filesWritten := 0
	for _, f := range r.File {
		name := f.Name
		if commonRoot != "" {
			name = strings.TrimPrefix(name, commonRoot)
			if name == "" {
				continue
			}
		}

		targetPath := filepath.Join(destDir, filepath.FromSlash(name))

		// Security: prevent path traversal
		if !strings.HasPrefix(targetPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return filesWritten, err
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			return filesWritten, err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return filesWritten, err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return filesWritten, err
		}
		filesWritten++
	}

	return filesWritten, nil
}

// extractTarGz extracts a .tar.gz archive to the destination directory.
func extractTarGz(archivePath, destDir string) (int, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	filesWritten := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return filesWritten, err
		}

		targetPath := filepath.Join(destDir, filepath.FromSlash(header.Name))

		// Security: prevent path traversal
		if !strings.HasPrefix(targetPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(targetPath, 0755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return filesWritten, err
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return filesWritten, err
			}
			_, err = io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return filesWritten, err
			}
			filesWritten++
		}
	}

	return filesWritten, nil
}
