package skillkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/util"
)

func ResolveSkillPath(skillRef, skillsRoot string) (string, error) {
	if skillRef == "" {
		return "", errors.New("Skill id or path is required.")
	}

	fullref, err := filepath.Abs(skillRef)
	if err != nil {
		return "", err
	}

	if util.DirectoryExists(fullref) || util.DirectoryExists(filepath.Join(fullref, "skill.yaml")) {
		return fullref, nil
	}

	if filepath.IsAbs(skillRef) {
		return fullref, nil
	}

	return filepath.Abs(filepath.Join(skillsRoot, skillRef))
}

type InvalidDataError struct {
	Message string
}

func (e *InvalidDataError) Error() string {
	return e.Message
}

func ResolvePackageFilePath(packageRoot, fileName string) (string, error) {
	if fileName == "" || filepath.IsAbs(fileName) {
		return "", &InvalidDataError{Message: fmt.Sprintf("Skill package file name must be relative: %s", fileName)}
	}

	fullRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return "", &InvalidDataError{Message: err.Error()}
	}

	fullPath, err := filepath.Abs(filepath.Join(fullRoot, fileName))
	if err != nil {
		return "", &InvalidDataError{Message: err.Error()}
	}

	relative, err := filepath.Rel(fullRoot, fullPath)
	if err != nil {
		return "", &InvalidDataError{Message: err.Error()}
	}

	if strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return "", &InvalidDataError{Message: fmt.Sprintf("Skill package file escapes package root: %s", fileName)}
	}

	return fullPath, nil
}

type SkillPackageReader struct{}

var SkillTemplateRequiredFiles []string = []string{
	"skill.yaml",
	"intent.md",
	"expectations.md",
	"workflow.yaml",
	"tools.yaml",
	"guardrails.md",
	"validation.md",
	"examples.md",
	"trace.md",
}

func (s *SkillPackageReader) Read(ctx context.Context, skillRef, skillsRoot string) (*SkillPackage, error) {
	packageRoot, err := ResolveSkillPath(skillRef, skillsRoot)
	if err != nil {
		return nil, err
	}

	var manifestPath = filepath.Join(packageRoot, "skill.yaml")
	if !util.FileExists(manifestPath) {
		return nil, fmt.Errorf("Skill manifest not found: %s", manifestPath)
	}

	manifest, err := SkillManifestRead(ctx, manifestPath)
	if err != nil {
		return nil, err
	}

	files := map[string]string{}
	for _, file := range SkillTemplateRequiredFiles {
		path, err := ResolvePackageFilePath(packageRoot, file)
		if err != nil {
			return nil, err
		}

		datas, err := os.ReadFile(path)
		if err == nil && len(datas) > 0 {
			files[file] = string(datas)
		}
	}

	return &SkillPackage{
		RootPath: packageRoot,
		Manifest: *manifest,
		Files:    files,
	}, nil
}

func (s *SkillPackageReader) List(ctx context.Context, skillsRoot string) ([]SkillPackage, error) {
	if !util.DirectoryExists(skillsRoot) {
		return []SkillPackage{}, nil
	}

	packages := []SkillPackage{}
	for _, directory := range util.EnumerateAllFiles(skillsRoot) {
		var manifestPath = filepath.Join(directory, "skill.yaml")
		if !util.FileExists(manifestPath) {
			continue
		}

		pkg, err := s.Read(ctx, directory, skillsRoot)
		if err != nil {
			var pathErr *os.PathError
			var invalidErr *InvalidDataError

			switch {
			case os.IsPermission(err):
				log.Printf("Skipping skill package at '%s' because access was denied: %v", directory, err)
			case errors.As(err, &pathErr) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF):
				log.Printf("Skipping skill package at '%s' because it could not be read: %v", directory, err)
			case errors.Is(err, invalidErr):
				log.Printf("Skipping skill package at '%s' because it is invalid: %v", directory, err)
			default:
				return nil, err
			}
		} else {
			packages = append(packages, *pkg)
		}
	}

	return packages, nil
}
