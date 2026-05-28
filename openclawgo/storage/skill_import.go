package storage

import (
	"errors"
	"os"
	"path/filepath"
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
