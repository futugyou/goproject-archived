package core

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func findDirectoriesCantainsFileName(candidatePath string, filename string) ([]string, error) {
	uniquePaths := make(map[string]bool)

	err := filepath.WalkDir(candidatePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if !d.IsDir() && d.Name() == filename {
			dir := filepath.Dir(path)

			if strings.TrimSpace(dir) != "" {
				uniquePaths[dir] = true
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	matches := make([]string, 0, len(uniquePaths))
	for path := range uniquePaths {
		matches = append(matches, path)
	}

	return matches, nil
}

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func isBlankP(s *string) bool {
	if s == nil {
		return true
	}
	return strings.TrimSpace(*s) == ""
}

func readStringArray(raw json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

func containsIgnoreCase(slice []string, val string) bool {
	target := strings.ToLower(val)
	for _, item := range slice {
		if strings.ToLower(item) == target {
			return true
		}
	}
	return false
}
