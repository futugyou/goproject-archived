package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type UserFolderHealthCheckResult struct {
	StorageRoot      string
	FoldersInspected int
	Findings         []string
}

func CleanUserFolderHealthCheckResult(storageRoot string, foldersInspected int) *UserFolderHealthCheckResult {
	return &UserFolderHealthCheckResult{
		StorageRoot:      storageRoot,
		FoldersInspected: foldersInspected,
		Findings:         []string{},
	}
}

func (u *UserFolderHealthCheckResult) HasFindings() bool {
	return len(u.Findings) > 0
}

type IUserFolderHealthCheck interface {
	Sweep(ctx context.Context, storageRoot string) (*UserFolderHealthCheckResult, error)
}

var _ IUserFolderHealthCheck = (*UserFolderHealthCheck)(nil)

var ExcludedFolderNames = map[string]struct{}{
	"agents": {},
	"models": {},
	"skills": {},
	"binary": {},
	// Internal credentials directory — owned by DataProtection, not
	// a user folder; sweep should not touch it.
	"dataprotection-keys": {},
	// Audit drop-folder added in W-4 commit #4. Treat like a scope
	// subfolder so the sweep doesn't false-positive against itself.
	"audit": {},
}

type UserFolderHealthCheck struct {
}

// Sweep implements [IUserFolderHealthCheck].
func (u *UserFolderHealthCheck) Sweep(ctx context.Context, storageRoot string) (*UserFolderHealthCheckResult, error) {
	if strings.TrimSpace(storageRoot) == "" {
		return &UserFolderHealthCheckResult{}, errors.New("storage root must be non-empty")
	}
	if err := ctx.Err(); err != nil {
		return &UserFolderHealthCheckResult{}, err
	}

	if _, err := os.Stat(storageRoot); os.IsNotExist(err) {
		// Nothing to sweep yet — first boot. Not an error.
		return CleanUserFolderHealthCheckResult(storageRoot, 0), nil
	}

	absRoot, err := filepath.Abs(storageRoot)
	if err != nil {
		return &UserFolderHealthCheckResult{}, fmt.Errorf("failed to get absolute path: %w", err)
	}
	normalizedRoot := filepath.Clean(absRoot)

	var findings []string
	inspected := 0

	entries, err := os.ReadDir(normalizedRoot)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EIO) {

			return CleanUserFolderHealthCheckResult(normalizedRoot, 0), nil
		}
		return &UserFolderHealthCheckResult{}, err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return &UserFolderHealthCheckResult{}, err
		}

		if !entry.IsDir() && (entry.Type()&os.ModeSymlink == 0) {
			continue
		}

		name := entry.Name()
		if name == "" {
			continue
		}

		if _, ok := ExcludedFolderNames[name]; ok {
			continue
		}

		childPath := filepath.Join(normalizedRoot, name)
		inspected++

		info, err := os.Lstat(childPath)
		if err != nil {
			if errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EIO) {
				continue
			}
			return &UserFolderHealthCheckResult{}, err
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		var finding string
		target, err := filepath.EvalSymlinks(childPath)
		if err != nil {
			finding = fmt.Sprintf("User-folder '%s' is a reparse point whose target could not be resolved: %v.", name, err)
		} else {
			targetAbs, _ := filepath.Abs(target)
			targetNormalized := filepath.Clean(targetAbs)

			inside := false
			if targetNormalized == normalizedRoot {
				inside = true
			} else {
				prefix := normalizedRoot
				if !strings.HasSuffix(prefix, string(filepath.Separator)) {
					prefix += string(filepath.Separator)
				}
				if strings.HasPrefix(targetNormalized, prefix) {
					inside = true
				}
			}

			if inside {
				continue
			}

			finding = fmt.Sprintf("User-folder '%s' is a reparse point whose target escapes the storage root: '%s'.", name, targetNormalized)
		}

		findings = append(findings, finding)
	}

	return &UserFolderHealthCheckResult{
		StorageRoot:      normalizedRoot,
		FoldersInspected: inspected,
		Findings:         findings,
	}, nil
}
