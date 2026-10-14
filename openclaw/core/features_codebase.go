package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/futugyou/openclaw/util"
	"github.com/google/uuid"
	"golang.org/x/mod/modfile"
)

var (
	mapEndpointPattern   = regexp.MustCompile(`(?:\b\w+\.)?Map(?P<method>Get|Post|Put|Delete|Patch)\s*\(\s*"(?P<path>[^"]+)?"`)
	httpAttributePattern = regexp.MustCompile(`\[(?P<attr>HttpGet|HttpPost|HttpPut|HttpDelete|HttpPatch)(?:Attribute)?(?:\s*\(\s*"(?P<path>[^"]+)"")?`)
	classPattern         = regexp.MustCompile(`\b(?:public|internal|private|sealed|abstract|static|\s)*\s*class\s+(?P<name>[A-Za-z_][A-Za-z0-9_]*)`)
)
var (
	mapEndpointPatternMethodIdx = mapEndpointPattern.SubexpIndex("method")
	mapEndpointPatternPathIdx   = mapEndpointPattern.SubexpIndex("path")
	httpAttributePatternAttrIdx = httpAttributePattern.SubexpIndex("attr")
	httpAttributePatternPathIdx = httpAttributePattern.SubexpIndex("path")
	hclassPatternNameIdx        = classPattern.SubexpIndex("name")
)

const GeneratorVersion string = "codebase-harness-map-mvp-1"
const MaxTextScanBytes int64 = 512 * 1024
const MaxHashBytes int64 = 8 * 1024 * 1024

type ScannedFile struct {
	FileInfo fs.FileInfo
	Depth    int
	RelPath  string
}

type stackItem struct {
	Directory string
	Depth     int
}

func normalizeOptions(input *CodebaseMapOptions) CodebaseMapOptions {
	if input == nil {
		input = &CodebaseMapOptions{}
	}

	category := NormalizeCategory(input.Category)

	options := CodebaseMapOptions{
		IncludeHashes:           input.IncludeHashes,
		IncludeRecentChanges:    input.IncludeRecentChanges,
		IncludeEndpoints:        input.IncludeEndpoints,
		IncludeToolSurfaces:     input.IncludeToolSurfaces,
		IncludeProviderSurfaces: input.IncludeProviderSurfaces,
		IncludeChannelSurfaces:  input.IncludeChannelSurfaces,
		IncludeConfigSurfaces:   input.IncludeConfigSurfaces,
		IncludeTests:            input.IncludeTests,
		IncludeDocs:             input.IncludeDocs,
		MaxFiles:                util.Clamp(input.MaxFiles, 1, 50000),
		MaxDepth:                util.Clamp(input.MaxDepth, 1, 64),
		RecentDays:              util.Clamp(input.RecentDays, 1, 3650),
		Category:                category,
	}

	switch category {
	case CodebaseMapCategoriesProjects:
		return withCategory(options, category, false, false, false, false, false, false)
	case CodebaseMapCategoriesEndpoints:
		return withCategory(options, category, true, false, false, false, false, false)
	case CodebaseMapCategoriesTools:
		return withCategory(options, category, false, true, false, false, false, false)
	case CodebaseMapCategoriesProviders:
		return withCategory(options, category, false, false, true, false, false, false)
	case CodebaseMapCategoriesChannels:
		return withCategory(options, category, false, false, false, true, false, false)
	case CodebaseMapCategoriesConfig:
		return withCategory(options, category, false, false, false, false, true, false)
	case CodebaseMapCategoriesTests:
		return withCategory(options, category, false, false, false, false, false, true)
	default:
		return options
	}
}

func NormalizeCategory(category string) string {
	value := strings.TrimSpace(category)
	if value == "" {
		return CodebaseMapCategoriesAll
	}
	value = strings.ToLower(value)

	switch value {
	case CodebaseMapCategoriesAll,
		CodebaseMapCategoriesProjects,
		CodebaseMapCategoriesEndpoints,
		CodebaseMapCategoriesTools,
		CodebaseMapCategoriesProviders,
		CodebaseMapCategoriesChannels,
		CodebaseMapCategoriesConfig,
		CodebaseMapCategoriesTests:
		return value
	default:
		return CodebaseMapCategoriesAll
	}
}

func resolveRoot(repositoryRoot string, diagnostics []CodebaseMapDiagnostic) string {
	root := strings.TrimSpace(repositoryRoot)
	var err error
	if root == "" {
		root, err = os.Getwd()
	}
	if err != nil {
		diagnostics = append(diagnostics, CodebaseMapDiagnostic{
			Severity:       CodebaseMapDiagnosticSeverityError,
			Code:           "root_invalid",
			Message:        fmt.Sprintf("Repository root '%s' is invalid: %s", root, err.Error()),
			Recommendation: "Pass a normal local filesystem path.",
		})
	}

	return root
}

func withCategory(
	options CodebaseMapOptions,
	category string,
	includeEndpoints bool,
	includeTools bool,
	includeProviders bool,
	includeChannels bool,
	includeConfig bool,
	includeTests bool,
) CodebaseMapOptions {
	options.IncludeEndpoints = includeEndpoints
	options.IncludeToolSurfaces = includeTools
	options.IncludeProviderSurfaces = includeProviders
	options.IncludeChannelSurfaces = includeChannels
	options.IncludeConfigSurfaces = includeConfig
	options.IncludeTests = includeTests
	options.Category = category
	return options
}

func buildMap(
	root string,
	repositoryName string,
	generatedAt time.Time,
	projects []CodebaseProject,
	modules []CodebaseModule,
	artifacts []CodebaseArtifact,
	endpoints []CodebaseEndpoint,
	tools []CodebaseToolSurface,
	providers []CodebaseProviderSurface,
	channels []CodebaseChannelSurface,
	configs []CodebaseConfigSurface,
	tests []CodebaseTestSurface,
	diagnostics []CodebaseMapDiagnostic,
	options *CodebaseMapOptions,
	recentPaths map[string]struct{},
) *CodebaseHarnessMap {
	warningCount := 0
	for _, item := range diagnostics {
		if item.Severity == CodebaseMapDiagnosticSeverityWarning || item.Severity == CodebaseMapDiagnosticSeverityError {
			warningCount++
		}
	}

	solutionFilesCount := 0
	sourceFilesCount := 0
	configFileCount := 0
	for _, item := range artifacts {
		if item.Kind == CodebaseMapArtifactKindsSolution {
			solutionFilesCount++
		}
		if item.Kind == CodebaseMapArtifactKindsSource {
			sourceFilesCount++
		}
		if item.Kind == CodebaseMapArtifactKindsConfig {
			configFileCount++
		}
	}

	testProjectsCount := 0
	for _, item := range projects {
		if item.IsTestProject {
			testProjectsCount++
		}
	}

	newUuid := strings.ReplaceAll(uuid.New().String(), "-", "")
	mapId := "cbhm_" + newUuid
	if len(mapId) > 20 {
		mapId = mapId[:20]
	}

	return &CodebaseHarnessMap{
		Id:               mapId,
		RepositoryRoot:   root,
		RepositoryName:   repositoryName,
		GeneratedAtUtc:   generatedAt,
		GeneratorVersion: GeneratorVersion,
		Summary: CodebaseMapSummary{
			SolutionFilesCount:   solutionFilesCount,
			ProjectFilesCount:    len(projects),
			SourceFilesCount:     sourceFilesCount,
			TestProjectsCount:    testProjectsCount,
			EndpointCount:        len(endpoints),
			ToolSurfaceCount:     len(tools),
			ChannelSurfaceCount:  len(channels),
			ProviderSurfaceCount: len(providers),
			ConfigFileCount:      configFileCount,
			RecentChangeCount:    len(recentPaths),
			WarningCount:         warningCount,
		},
		Projects:         projects,
		Modules:          modules,
		Artifacts:        artifacts,
		Endpoints:        endpoints,
		ToolSurfaces:     tools,
		ProviderSurfaces: providers,
		ChannelSurfaces:  channels,
		ConfigSurfaces:   configs,
		TestSurfaces:     tests,
		Diagnostics:      diagnostics,
		Tags:             []string{options.Category},
		Metadata: map[string]string{
			"category": options.Category,
			"maxFiles": strconv.Itoa(options.MaxFiles),
			"maxDepth": strconv.Itoa(options.MaxDepth),
		},
	}
}
func enumerateFiles(ctx context.Context, root string, options *CodebaseMapOptions, diagnostics *[]CodebaseMapDiagnostic) ([]ScannedFile, error) {
	var results []ScannedFile
	stack := []stackItem{{Directory: root, Depth: 0}}

	for len(stack) > 0 {
		// Check for cancellation
		if err := ctx.Err(); err != nil {
			return results, err
		}

		// Pop from stack
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Read directory (os.ReadDir returns entries sorted by filename alphabetically)
		entries, err := os.ReadDir(current.Directory)
		if err != nil {
			if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {
				rel := relativePath(root, current.Directory)
				*diagnostics = append(*diagnostics, CodebaseMapDiagnostic{
					Severity: CodebaseMapDiagnosticSeverityWarning,
					Code:     "directory_unreadable",
					Message:  "Could not read directory '" + rel + "': " + err.Error(),
					Path:     rel,
				})
				continue
			}
			rel := relativePath(root, current.Directory)
			*diagnostics = append(*diagnostics, CodebaseMapDiagnostic{
				Severity: CodebaseMapDiagnosticSeverityWarning,
				Code:     "directory_unreadable",
				Message:  "Could not read directory '" + rel + "': " + err.Error(),
				Path:     rel,
			})
			continue
		}

		var files []fs.DirEntry
		var directories []fs.DirEntry

		for _, entry := range entries {
			if entry.IsDir() {
				directories = append(directories, entry)
			} else {
				files = append(files, entry)
			}
		}

		// Process files
		for _, file := range files {
			if len(results) >= options.MaxFiles {
				*diagnostics = append(*diagnostics, CodebaseMapDiagnostic{
					Severity:       CodebaseMapDiagnosticSeverityWarning,
					Code:           "max_files_reached",
					Message:        "Stopped scanning after maximum file limit reached.",
					Recommendation: "Increase max-files configuration for a larger repository map.",
				})
				return results, nil
			}

			fullPath := filepath.Join(current.Directory, file.Name())
			info, err := file.Info()
			if err != nil {
				continue
			}

			results = append(results, ScannedFile{
				FileInfo: info,
				Depth:    current.Depth,
				RelPath:  relativePath(root, fullPath),
			})
		}

		if current.Depth >= options.MaxDepth {
			continue
		}

		// Push directories to stack in reverse order to preserve alphabetical pop order
		for i := len(directories) - 1; i >= 0; i-- {
			child := directories[i]
			if shouldSkipDirectory(child.Name()) {
				continue
			}

			fullPath := filepath.Join(current.Directory, child.Name())

			if isReparsePoint(fullPath) {
				rel := relativePath(root, fullPath)
				*diagnostics = append(*diagnostics, CodebaseMapDiagnostic{
					Severity:       CodebaseMapDiagnosticSeverityWarning,
					Code:           "directory_reparse_point_skipped",
					Message:        "Skipped symlink or reparse-point directory '" + rel + "'.",
					Path:           rel,
					Recommendation: "Codebase map scanning does not follow symlinked directories.",
				})
				continue
			}

			stack = append(stack, stackItem{
				Directory: fullPath,
				Depth:     current.Depth + 1,
			})
		}
	}

	return results, nil
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return normalizeSlashes(path)
	}
	return normalizeSlashes(rel)
}

func normalizeSlashes(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func isReparsePoint(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return true // Treat unreadable or error-throwing paths safely as reparse points/skipped
	}
	return info.Mode()&os.ModeSymlink != 0
}

func shouldSkipDirectory(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "bin", "obj", "node_modules", ".idea", ".vs", ".vscode", "TestResults", "artifacts":
		return true
	}
	return false
}

func collectRecentPaths(files []ScannedFile, options *CodebaseMapOptions) map[string]struct{} {
	cutoff := time.Now().Add(time.Duration(-options.RecentDays) * time.Hour * 24)

	result := map[string]struct{}{}
	for _, file := range files {
		if safeLastModified(file.FileInfo).After(cutoff) {
			if _, ok := result[file.RelPath]; !ok {
				result[file.RelPath] = struct{}{}
			}
		}
	}
	return result
}

func safeLastModified(fileInfo fs.FileInfo) time.Time {
	return fileInfo.ModTime()
}

func stableId(prefix, key string) string {
	sum := sha256.Sum256([]byte(key))
	hash := hex.EncodeToString(sum[:])
	if len(hash) > 8 {
		hash = hash[:8]
	}
	hash = strings.ToLower(hash)

	base := filepath.Base(key)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	name := sanitizeId(nameWithoutExt)

	return fmt.Sprintf("%s_%s_%s", prefix, name, hash)
}

func sanitizeId(value string) string {
	var sb strings.Builder

	for _, ch := range value {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			sb.WriteRune(unicode.ToLower(ch))
		} else {
			current := sb.String()
			if len(current) == 0 || current[len(current)-1] != '_' {
				sb.WriteRune('_')
			}
		}
	}

	id := strings.Trim(sb.String(), "_")
	if id == "" {
		return "item"
	}
	return id
}

func detectProjects(root string, files []ScannedFile, diagnostics *[]CodebaseMapDiagnostic) []CodebaseProject {
	var projects []CodebaseProject

	for _, file := range files {
		if filepath.Base(file.RelPath) != "go.mod" {
			continue
		}

		dirPath := filepath.Dir(file.RelPath)
		if dirPath == "." {
			dirPath = ""
		}

		name := filepath.Base(filepath.Dir(filepath.Join(root, file.RelPath)))
		if dirPath == "" {
			name = filepath.Base(root)
		}

		fullPath := filepath.Join(root, file.RelPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			*diagnostics = append(*diagnostics, CodebaseMapDiagnostic{
				Severity:       CodebaseMapDiagnosticSeverityWarning,
				Code:           "project_parse_failed",
				Message:        "Could not read go.mod file '" + file.RelPath + "': " + err.Error(),
				Path:           file.RelPath,
				Recommendation: "Inspect the file permissions or existence.",
			})
			projects = append(projects, CodebaseProject{
				Id:          stableId("project", file.RelPath),
				Name:        name,
				Path:        file.RelPath,
				ProjectType: "unknown",
			})
			continue
		}

		parsedMod, err := modfile.Parse(file.RelPath, content, nil)
		if err != nil {
			*diagnostics = append(*diagnostics, CodebaseMapDiagnostic{
				Severity:       CodebaseMapDiagnosticSeverityWarning,
				Code:           "project_parse_failed",
				Message:        "Could not parse go.mod '" + file.RelPath + "': " + err.Error(),
				Path:           file.RelPath,
				Recommendation: "Inspect the go.mod syntax.",
			})
			projects = append(projects, CodebaseProject{
				Id:          stableId("project", file.RelPath),
				Name:        name,
				Path:        file.RelPath,
				ProjectType: "unknown",
			})
			continue
		}

		if parsedMod.Module != nil && parsedMod.Module.Mod.Path != "" {
			name = parsedMod.Module.Mod.Path
		}

		goVersion := ""
		if parsedMod.Go != nil {
			goVersion = parsedMod.Go.Version
		}

		var packages []string
		for _, req := range parsedMod.Require {
			if !req.Indirect {
				packages = append(packages, req.Mod.Path+" "+req.Mod.Version)
			}
		}
		sort.Slice(packages, func(i, j int) bool {
			return strings.ToLower(packages[i]) < strings.ToLower(packages[j])
		})

		isTest := strings.Contains(strings.ToLower(file.RelPath), "test")
		var tags []string
		if isTest {
			tags = []string{"test"}
		}

		projects = append(projects, CodebaseProject{
			Id:                stableId("project", file.RelPath),
			Name:              name,
			Path:              file.RelPath,
			ProjectType:       "go-module",
			GoVersion:         goVersion,
			IsTestProject:     isTest,
			PackageReferences: packages,
			Tags:              tags,
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Path) < strings.ToLower(projects[j].Path)
	})

	return projects
}

type CodebaseHarnessMapService struct{}

func (c *CodebaseHarnessMapService) Generate(ctx context.Context, repositoryRoot string, options *CodebaseMapOptions) (*CodebaseHarnessMap, error) {
	started := time.Now()
	_ = normalizeOptions(options)

	diagnostics := []CodebaseMapDiagnostic{}
	root := resolveRoot(repositoryRoot, diagnostics)
	repositoryName := filepath.Base(root)

	if _, err := os.Stat(root); os.IsNotExist(err) {
		diagnostics = append(diagnostics, CodebaseMapDiagnostic{
			Severity:       CodebaseMapDiagnosticSeverityError,
			Code:           "root_missing",
			Message:        fmt.Sprintf("Repository root '%s' does not exist.", root),
			Recommendation: "Pass --root with an existing repository or workspace path.",
		})

		return buildMap(root, repositoryName, started, nil, nil, nil, nil, nil, nil, nil, nil, nil, diagnostics, options, nil), nil
	}

	files, err := enumerateFiles(ctx, root, options, &diagnostics)
	if err != nil {
		return nil, err
	}

	recentPaths := map[string]struct{}{}
	if options.IncludeRecentChanges {
		recentPaths = collectRecentPaths(files, options)
	}

	projects := detectProjects(root, files, &diagnostics)
	// TODO:
	return buildMap(root, repositoryName, started, projects, nil, nil, nil, nil, nil, nil, nil, nil, diagnostics, options, recentPaths), nil
}
