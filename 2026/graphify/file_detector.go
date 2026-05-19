package graphify

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

var _ IPipelineStage[FileDetectorOptions, []DetectedFile] = (*FileDetector)(nil)

var (
	FileDetectorCodeExtensions = map[string]struct{}{
		".py": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {}, ".go": {}, ".rs": {}, ".java": {}, ".c": {}, ".cpp": {}, ".h": {}, ".hpp": {},
		".rb": {}, ".cs": {}, ".kt": {}, ".scala": {}, ".php": {}, ".swift": {}, ".r": {}, ".lua": {}, ".sh": {}, ".bash": {}, ".ps1": {},
		".yaml": {}, ".yml": {}, ".json": {}, ".toml": {}, ".xml": {},
	}

	FileDetectorDocumentationExtensions = map[string]struct{}{
		".md": {}, ".txt": {}, ".rst": {}, ".adoc": {},
	}

	FileDetectorMediaExtensions = map[string]struct{}{
		".pdf": {}, ".png": {}, ".jpg": {}, ".jpeg": {}, ".webp": {}, ".gif": {}, ".svg": {},
	}

	FileDetectorExtensionToLanguage = map[string]string{
		".py":    "Python",
		".ts":    "TypeScript",
		".tsx":   "TypeScript",
		".js":    "JavaScript",
		".jsx":   "JavaScript",
		".go":    "Go",
		".rs":    "Rust",
		".java":  "Java",
		".c":     "C",
		".cpp":   "C++",
		".h":     "C",
		".hpp":   "C++",
		".rb":    "Ruby",
		".cs":    "CSharp",
		".kt":    "Kotlin",
		".scala": "Scala",
		".php":   "PHP",
		".swift": "Swift",
		".r":     "R",
		".lua":   "Lua",
		".sh":    "Shell",
		".bash":  "Shell",
		".ps1":   "PowerShell",
		".yaml":  "YAML",
		".yml":   "YAML",
		".json":  "JSON",
		".toml":  "TOML",
		".xml":   "XML",
		".md":    "Markdown",
		".txt":   "Text",
		".rst":   "ReStructuredText",
		".adoc":  "AsciiDoc",
		".pdf":   "PDF",
		".png":   "PNG",
		".jpg":   "JPEG",
		".jpeg":  "JPEG",
		".webp":  "WebP",
		".gif":   "GIF",
		".svg":   "SVG",
	}

	FileDetectorSkipDirectories = map[string]struct{}{
		"venv": {}, ".venv": {}, "env": {}, ".env": {}, "node_modules": {}, "__pycache__": {}, ".git": {},
		"dist": {}, "build": {}, "target": {}, "out": {}, "bin": {}, "obj": {}, "site-packages": {}, "lib64": {},
		".pytest_cache": {}, ".mypy_cache": {}, ".ruff_cache": {}, ".tox": {}, ".eggs": {},
	}
)

type FileDetector struct {
}

// Execute implements [IPipelineStage].
func (f *FileDetector) Execute(ctx context.Context, input FileDetectorOptions) (*[]DetectedFile, error) {
	detectedFiles := []DetectedFile{}

	if _, err := os.Stat(input.RootPath); err != nil {
		return &detectedFiles, err
	}

	rootPath := filepath.Dir(input.RootPath)

	// Validate root scan directory
	validator := NewInputValidator()
	pathValidation := validator.ValidatePath(rootPath, "")
	if !pathValidation.IsValid {
		return &detectedFiles, fmt.Errorf("Invalid root path: %s", strings.Join(pathValidation.Errors, "; "))
	}

	var gitTrackedFiles map[string]struct{}
	var err error
	if input.RespectGitIgnore {
		if gitTrackedFiles, err = f.getGitTrackedFiles(ctx, rootPath); err != nil {
			return &detectedFiles, err
		}
	}

	fileCh := f.enumerateFiles(ctx, rootPath, input, gitTrackedFiles)

	workerCount := input.WorkerCount
	batchSize := input.BatchSize

	if workerCount <= 0 {
		workerCount = 5
	}

	if batchSize <= 0 {
		batchSize = 10
	}

	type result struct {
		file DetectedFile
		err  error
	}

	batchCh := make(chan []string)
	resultCh := make(chan result)

	var wg sync.WaitGroup

	go func() {
		defer close(batchCh)
		batch := make([]string, 0, batchSize)
		for filePath := range fileCh {
			batch = append(batch, filePath)
			if len(batch) >= batchSize {
				batchCh <- batch
				batch = make([]string, 0, batchSize)
			}
		}
		if len(batch) > 0 {
			batchCh <- batch
		}
	}()

	for range workerCount {
		wg.Go(func() {
			for batch := range batchCh {
				fmt.Println("Processing batch:", batch)

				for _, filePath := range batch {
					df, err := f.processFile(ctx, filePath, rootPath, input)
					if err != nil {
						select {
						case resultCh <- result{err: err}:
						case <-ctx.Done():
							return
						}
						continue
					}

					select {
					case resultCh <- result{file: *df}:
					case <-ctx.Done():
						return
					}
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for r := range resultCh {
		if r.err != nil {
			return &detectedFiles, r.err
		}
		detectedFiles = append(detectedFiles, r.file)
	}

	slices.SortFunc(detectedFiles, func(a, b DetectedFile) int {
		return cmp.Compare(a.RelativePath, b.RelativePath)
	})

	return &detectedFiles, nil
}

func (f *FileDetector) getGitTrackedFiles(ctx context.Context, rootPath string) (map[string]struct{}, error) {
	trackedFiles := map[string]struct{}{}
	cmd := exec.CommandContext(ctx, "git", "ls-files")
	cmd.Dir = rootPath

	outputBytes, err := cmd.Output()
	if err != nil {
		return trackedFiles, fmt.Errorf("failed to execute git command: %w", err)
	}

	output := string(outputBytes)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {

		var trimmed = strings.TrimSpace(line)
		if len(line) > 0 {
			var fullPath = filepath.Join(rootPath, trimmed)
			trackedFiles[fullPath] = struct{}{}
		}
	}

	return trackedFiles, nil
}

func (f *FileDetector) isSkipDirectory(dirName string) bool {
	dirName = strings.ToLower(dirName)
	if _, ok := FileDetectorSkipDirectories[dirName]; ok {
		return true
	}

	if strings.HasSuffix(dirName, "_venv") || strings.HasSuffix(dirName, "_env") || strings.HasSuffix(dirName, ".egg-info") {
		return true
	}
	return false
}

func (f *FileDetector) classifyFile(extension string) FileCategory {
	if _, ok := FileDetectorCodeExtensions[extension]; ok {
		return FileCategoryCode
	}

	if _, ok := FileDetectorDocumentationExtensions[extension]; ok {
		return FileCategoryDocumentation
	}

	if _, ok := FileDetectorMediaExtensions[extension]; ok {
		return FileCategoryMedia
	}

	return FileCategoryUnkwon
}

func (f *FileDetector) processFile(_ context.Context, filePath, rootPath string, options FileDetectorOptions) (*DetectedFile, error) {
	var fileInfo os.FileInfo
	var err error
	if fileInfo, err = os.Stat(filePath); err != nil {
		return nil, err
	}

	if fileInfo.Size() > options.MaxFileSizeBytes {
		return nil, fmt.Errorf("file size is too big")
	}

	var extension = strings.ToLower(filepath.Ext(filePath))

	if len(options.IncludeExtensions) > 0 && !slices.Contains(options.IncludeExtensions, extension) {
		return nil, fmt.Errorf("file is not include")
	}

	var category = f.classifyFile(extension)
	if category == FileCategoryUnkwon {
		return nil, fmt.Errorf("file type is not supported")
	}

	relativePath, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		return nil, err
	}

	if len(options.ExcludePatterns) > 0 {
		relativePathLower := strings.ToLower(relativePath)
		for _, pattern := range options.ExcludePatterns {
			patternLower := strings.ToLower(pattern)
			matched, err := filepath.Match(patternLower, relativePathLower)
			if err == nil && matched {
				return nil, fmt.Errorf("include the excluded directory.")
			}
		}
	}

	var language = strings.TrimRight(extension, ".")
	if lang, ok := FileDetectorExtensionToLanguage[extension]; ok {
		language = lang
	}

	return &DetectedFile{
		FilePath:     filePath,
		FileName:     fileInfo.Name(),
		Extension:    extension,
		Language:     language,
		Category:     category,
		SizeBytes:    fileInfo.Size(),
		RelativePath: relativePath,
	}, nil
}

func (f *FileDetector) enumerateFiles(ctx context.Context, rootPath string, _ FileDetectorOptions, gitTrackedFiles map[string]struct{}) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out)
		queue := []string{rootPath}

		for len(queue) > 0 {
			select {
			case <-ctx.Done():
				return
			default:
			}

			currentDir := queue[0]
			queue = queue[1:]

			entries, err := os.ReadDir(currentDir)
			if err != nil {
				if os.IsPermission(err) {
					continue
				}
				continue
			}

			for _, entry := range entries {
				name := entry.Name()
				fullPath := filepath.Join(currentDir, name)
				if strings.HasPrefix(name, ".") {
					continue
				}

				if entry.IsDir() {
					if f.isSkipDirectory(name) {
						continue
					}

					info, err := entry.Info()
					if err != nil {
						continue
					}

					if info.Mode()&os.ModeSymlink != 0 {
						continue
					}

					resolvedDir, err := filepath.EvalSymlinks(fullPath)
					if err != nil {
						continue
					}
					resolvedDir, err = filepath.Abs(resolvedDir)
					if err != nil {
						continue
					}
					if !strings.HasPrefix(resolvedDir, rootPath) {
						continue
					}

					queue = append(queue, fullPath)
				} else {
					info, err := entry.Info()
					if err != nil {
						continue
					}
					if info.Mode()&os.ModeSymlink != 0 {
						continue
					}

					resolvedFile, err := filepath.Abs(fullPath)
					if err != nil {
						continue
					}
					if !strings.HasPrefix(resolvedFile, rootPath) {
						continue
					}

					if gitTrackedFiles != nil {
						if _, ok := gitTrackedFiles[resolvedFile]; !ok {
							continue
						}
					}

					out <- resolvedFile
				}
			}
		}
	}()

	return out
}
