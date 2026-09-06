package skillkit

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/util"
)

func ValidateSinglePathSegment(value, parameterName string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("Value cannot be empty.")
	}

	if filepath.IsAbs(value) ||
		strings.Contains(value, string(os.PathSeparator)) ||
		strings.Contains(value, "/") {
		return "", fmt.Errorf("Value must be a single path segment: %s", parameterName)
	}

	return value, nil
}

type SkillPackageWriter struct {
	renderer SkillTemplateRenderer
}

func NewSkillPackageWriter(renderer SkillTemplateRenderer) *SkillPackageWriter {
	return &SkillPackageWriter{renderer: renderer}
}

func (s *SkillPackageWriter) CreateZip(ctx context.Context, pkg SkillPackage, packagesRoot string, force bool) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if err := os.MkdirAll(packagesRoot, 0755); err != nil {
		return "", err
	}

	manifestId, err := ValidateSinglePathSegment(pkg.Manifest.Id, "Id")
	if err != nil {
		return "", err
	}

	manifestVersion, err := ValidateSinglePathSegment(pkg.Manifest.Version, "Version")
	if err != nil {
		return "", err
	}
	var packageFileName = fmt.Sprintf("%s-%s.zip", manifestId, manifestVersion)
	var zipPath = filepath.Join(packagesRoot, packageFileName)
	if util.FileExists(zipPath) {
		if !force {
			return "", fmt.Errorf("Package already exists: %s. Use --force to overwrite.", zipPath)
		}
		os.Remove(zipPath)
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	for _, file := range SkillTemplateRequiredFiles {
		source := filepath.Join(packagesRoot, file)
		if info, err := os.Stat(source); err == nil && !info.IsDir() {
			if err := util.AddFileToZip(archive, source, file); err != nil {
				return "", err
			}
		}
	}
	return zipPath, nil
}

func (s *SkillPackageWriter) GenerateMissing(ctx context.Context, pkg SkillPackage, force bool) error {
	for file, content := range s.renderer.RenderFiles(pkg.Manifest) {
		target, err := ResolvePackageFilePath(pkg.RootPath, file)
		if err != nil || (util.FileExists(target) && !force) {
			continue
		}

		util.SaveFile(ctx, target, content)
	}

	return nil
}

func (s *SkillPackageWriter) Create(ctx context.Context, manifest SkillManifest, skillsRoot string, force bool) (string, error) {
	skillDirectoryName, err := ValidateSinglePathSegment(manifest.Id, "Id")
	if err != nil {
		return "", err
	}
	var packageRoot = filepath.Join(skillsRoot, skillDirectoryName)

	if util.DirectoryExists(packageRoot) && !force {
		return "", fmt.Errorf("Skill already exists: %s. Use --force to overwrite.", packageRoot)
	}

	os.MkdirAll(skillsRoot, 0755)

	if util.DirectoryExists(packageRoot) && force {
		os.Remove(packageRoot)
	}

	os.MkdirAll(packageRoot, 0755)

	for file, content := range s.renderer.RenderFiles(manifest) {
		path, err := ResolvePackageFilePath(packageRoot, file)
		if err == nil && len(path) > 0 {
			util.SaveFile(ctx, path, content)
		}
	}

	return packageRoot, nil
}
