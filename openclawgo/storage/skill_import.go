package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const StorageOptionsSectionName = "Storage"

type StorageOptions struct {
	RootPath              string
	BinaryFolderName      string
	ModelsFolderName      string
	AgentsFolderName      string
	SkillsFolderName      string
	ModelMaxTotalBytes    int64
	ModelMaxPerFileBytes  int64
	UserMaxPerFolderBytes int64
	UserMaxTotalBytes     int64
	BinaryArtifactsPath   string
	ModelsPath            string
	AgentsPath            string
	SkillsPath            string
}

func DefaultStorageOptions() *StorageOptions {
	o := &StorageOptions{
		RootPath:              "./openclaw",
		BinaryFolderName:      "binary",
		ModelsFolderName:      "models",
		AgentsFolderName:      "agents",
		SkillsFolderName:      "skills",
		ModelMaxTotalBytes:    50 * 1024 * 1024 * 1024,
		ModelMaxPerFileBytes:  20 * 1024 * 1024 * 1024,
		UserMaxPerFolderBytes: 5 * 1024 * 1024 * 1024,
		UserMaxTotalBytes:     25 * 1024 * 1024 * 1024,
		BinaryArtifactsPath:   "./openclaw/binary",
		ModelsPath:            "./openclaw/models",
		AgentsPath:            "./openclaw/agents",
		SkillsPath:            "./openclaw/skills",
	}
	return o
}

func (s *StorageOptions) BinaryFolderForTool(toolName string) string {
	var folder = filepath.Join(s.BinaryArtifactsPath, toolName)
	os.Mkdir(folder, 0755)
	return folder
}

func (s *StorageOptions) SanitizeAgentName(value string) (string, error) {
	if len(value) == 0 {
		return "", errors.New("agent name cannot be null")
	}

	r := strings.NewReplacer(
		"..", "_",
		"/", "_",
		"\\", "_",
		"\x00", "_",
	)
	sanitized := r.Replace(value)

	if len(sanitized) == 0 || allCharsAre(sanitized, '_') || allCharsAre(sanitized, '.') {
		return "", errors.New("invalid agent name (becomes empty after sanitization")
	}

	return sanitized, nil
}

func allCharsAre(s string, ch byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ch {
			return false
		}
	}
	return true
}

func (s *StorageOptions) AgentFolderForName(agentName string) (string, error) {
	sanitizedName, err := s.SanitizeAgentName(agentName)
	if err != nil {
		return "", err
	}

	var primaryPath = filepath.Join(s.AgentsPath, sanitizedName)
	err = os.Mkdir(primaryPath, 0755)
	if err != nil {
		return "", err
	}

	return primaryPath, nil
}

type SkillImportService struct {
	options *StorageOptions
}

func NewSkillImportService(options *StorageOptions) *SkillImportService {
	if options == nil {
		options = DefaultStorageOptions()
	}

	return &SkillImportService{
		options: options,
	}
}

func (s *SkillImportService) ImportSingleFile(ctx context.Context, fileName string, reader io.Reader) (string, error) {
	if strings.TrimSpace(fileName) == "" {
		return "", errors.New("file name cannot be empty")
	}

	if !strings.HasSuffix(strings.ToLower(fileName), ".md") {
		return "", errors.New("must be .md file")
	}

	skillName := strings.TrimSuffix(filepath.Base(fileName), ".md")

	skillPath := filepath.Join(
		s.options.SkillsPath,
		skillName,
		"SKILL.md",
	)

	if err := os.MkdirAll(filepath.Dir(skillPath), 0755); err != nil {
		return "", err
	}

	file, err := os.Create(skillPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return "", err
	}

	return skillName, nil
}

func (si *SkillImportService) ImportFolder(zipData []byte) (string, error) {
	reader := bytes.NewReader(zipData)
	size := int64(len(zipData))

	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return "", fmt.Errorf("invalid or corrupt zip file: %w", err)
	}

	if len(archive.File) == 0 {
		return "", errors.New("zip archive is empty")
	}

	var skillName string
	hasSkillMdAtRoot := false

	for _, f := range archive.File {
		if strings.EqualFold(f.Name, "SKILL.md") {
			hasSkillMdAtRoot = true
			break
		}
	}

	if hasSkillMdAtRoot {
		skillName = "imported-skill"
		for _, f := range archive.File {
			if strings.Contains(f.Name, "/") {
				skillName = strings.Split(f.Name, "/")[0]
				break
			}
		}
	} else {
		var skillMdEntries []*zip.File
		for _, f := range archive.File {
			baseName := filepath.Base(f.Name)
			if strings.EqualFold(baseName, "SKILL.md") && strings.Contains(f.Name, "/") {
				skillMdEntries = append(skillMdEntries, f)
			}
		}

		if len(skillMdEntries) == 0 {
			return "", errors.New("zip must contain SKILL.md at the root or in a single subfolder")
		}
		if len(skillMdEntries) > 1 {
			return "", errors.New("zip must contain only one skill folder (only one SKILL.md allowed)")
		}

		skillName = strings.Split(skillMdEntries[0].Name, "/")[0]
	}

	if err := si.ValidateSkillNameFormat(skillName); err != nil {
		return "", err
	}

	if !si.ValidateSkillName(skillName) {
		return "", fmt.Errorf("skill '%s' already exists. Delete or rename existing skill first", skillName)
	}

	for _, f := range archive.File {
		baseName := filepath.Base(f.Name)
		if strings.EqualFold(baseName, "SKILL.md") || f.FileInfo().IsDir() {
			continue
		}

		if strings.HasSuffix(strings.ToLower(baseName), ".yml") || strings.HasSuffix(strings.ToLower(baseName), ".yaml") {
			content, err := si.readZipFileContent(f)
			if err != nil {
				return "", err
			}
			if err := si.validateYamlSyntax(content, baseName); err != nil {
				return "", err
			}
		} else if strings.HasSuffix(strings.ToLower(baseName), ".json") {
			content, err := si.readZipFileContent(f)
			if err != nil {
				return "", err
			}
			if err := si.validateJsonSyntax(content, baseName); err != nil {
				return "", err
			}
		}
	}

	skillPath := filepath.Join(si.options.SkillsPath, skillName)
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		return "", fmt.Errorf("invalid or corrupt zip file: %w", err)
	}

	for _, f := range archive.File {
		if strings.HasPrefix(f.Name, skillName+"/") {
			relativePath := f.Name[len(skillName)+1:]
			if relativePath == "" {
				continue
			}

			entryPath := filepath.Join(skillPath, relativePath)

			if f.FileInfo().IsDir() {
				os.MkdirAll(entryPath, 0755)
				continue
			}

			os.MkdirAll(filepath.Dir(entryPath), 0755)
			if err := si.extractToFile(f, entryPath); err != nil {
				return "", err
			}

		} else if !strings.Contains(f.Name, "/") && !strings.EqualFold(f.Name, "SKILL.md") {
			if f.FileInfo().IsDir() {
				continue
			}
			entryPath := filepath.Join(skillPath, f.Name)
			if err := si.extractToFile(f, entryPath); err != nil {
				return "", err
			}

		} else if strings.EqualFold(f.Name, "SKILL.md") && !strings.Contains(f.Name, "/") {
			entryPath := filepath.Join(skillPath, "SKILL.md")
			if err := si.extractToFile(f, entryPath); err != nil {
				return "", err
			}
		}
	}

	return skillName, nil
}

func (si *SkillImportService) ValidateSkillName(name string) bool {
	if _, err := os.Stat(si.options.SkillsPath); os.IsNotExist(err) {
		return true
	}

	skillPath := filepath.Join(si.options.SkillsPath, name)
	skillFile := filepath.Join(si.options.SkillsPath, name+".md")

	_, errPath := os.Stat(skillPath)
	_, errFile := os.Stat(skillFile)

	return os.IsNotExist(errPath) && os.IsNotExist(errFile)
}

func (si *SkillImportService) ValidateSkillNameFormat(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("skill name cannot be empty")
	}

	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return errors.New("skill name cannot contain path traversal characters (.., /, \\)")
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-]+$`, name)

	if !matched {
		return errors.New("skill name must contain only alphanumeric characters and hyphens")
	}

	return nil
}

func (si *SkillImportService) validateYamlSyntax(content []byte, fileName string) error {
	if len(bytes.TrimSpace(content)) == 0 {
		return fmt.Errorf("subfile '%s' has invalid syntax: file content is empty", fileName)
	}
	return nil
}

func (si *SkillImportService) validateJsonSyntax(content []byte, fileName string) error {
	var js json.RawMessage
	if err := json.Unmarshal(content, &js); err != nil {
		return fmt.Errorf("subfile '%s' has invalid JSON syntax: %w", fileName, err)
	}
	return nil
}

func (si *SkillImportService) readZipFileContent(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (si *SkillImportService) extractToFile(f *zip.File, targetPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return err
	}
	return nil
}
