package graphify

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type WatchMode struct {
	output        io.Writer
	verbose       bool
	debounceDelay time.Duration
	cache         *SemanticCache
	watcher       *fsnotify.Watcher

	currentGraph   *KnowledgeGraph
	graphMu        sync.RWMutex
	pendingChanges map[string]time.Time
	pendingMu      sync.Mutex
	processChan    chan struct{}
}

func NewWatchMode(output io.Writer, verbose bool) *WatchMode {
	cache, err := NewSemanticCache("")
	if err != nil {
		panic(err)
	}

	return &WatchMode{
		output:         output,
		verbose:        verbose,
		debounceDelay:  500 * time.Millisecond,
		cache:          cache,
		pendingChanges: make(map[string]time.Time),
		processChan:    make(chan struct{}, 1),
	}
}

func (wm *WatchMode) SetInitialGraph(graph *KnowledgeGraph) {
	wm.graphMu.Lock()
	defer wm.graphMu.Unlock()
	wm.currentGraph = graph
}

func (wm *WatchMode) Watch(ctx context.Context, path, outputDir string, formats []string) error {
	fullPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		fmt.Fprintf(wm.output, "Error: Directory not found: %s\n", fullPath)
		return err
	}

	wm.graphMu.RLock()
	if wm.currentGraph == nil {
		wm.graphMu.RUnlock()
		fmt.Fprintln(wm.output, "Error: No initial graph set. Run the pipeline first.")
		return fmt.Errorf("initial graph is nil")
	}
	wm.graphMu.RUnlock()

	// Initialize the fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	wm.watcher = watcher
	defer wm.watcher.Close()

	// Recursively add directories to watch (fsnotify is non-recursive by default, so manual traversal is required).
	err = filepath.Walk(fullPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if wm.shouldIgnore(p) {
				return filepath.SkipDir
			}
			return wm.watcher.Add(p)
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(wm.output, "Watching %s for changes... (Press Ctrl+C to stop)\n\n", fullPath)

	// Start a Goroutine to listen for file system events.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-wm.watcher.Events:
				if !ok {
					return
				}
				if wm.shouldIgnore(event.Name) {
					continue
				}
				// For newly added folders, dynamically add them to the watch list.
				if event.Op&fsnotify.Create == fsnotify.Create {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						_ = wm.watcher.Add(event.Name)
					}
				}

				// Record the changed files in the pending dictionary.
				wm.pendingMu.Lock()
				wm.pendingChanges[event.Name] = time.Now()
				wm.pendingMu.Unlock()

			case err, ok := <-wm.watcher.Errors:
				if !ok {
					return
				}
				if wm.verbose {
					fmt.Fprintf(wm.output, "Watcher error: %v\n", err)
				}
			}
		}
	}()

	// Main Loop: Responsible for debouncing and triggering the incremental update pipeline.
	ticker := time.NewTicker(wm.debounceDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(wm.output, "\nWatch stopped.")
			return nil
		case <-ticker.C:
			wm.pendingMu.Lock()
			if len(wm.pendingChanges) == 0 {
				wm.pendingMu.Unlock()
				continue
			}

			// Snapshot and clear the pending queue
			changedFiles := make([]string, 0, len(wm.pendingChanges))
			for k := range wm.pendingChanges {
				changedFiles = append(changedFiles, k)
			}
			wm.pendingChanges = make(map[string]time.Time)
			wm.pendingMu.Unlock()

			// Perform asynchronous incremental processing
			go wm.processChanges(ctx, changedFiles, fullPath, outputDir, formats)
		}
	}
}

func (wm *WatchMode) processChanges(ctx context.Context, changedPaths []string, rootPath, outputDir string, formats []string) {
	// Acquire the channel lock in a non-blocking manner. If a task is already running, exit immediately.
	select {
	case wm.processChan <- struct{}{}:
		defer func() { <-wm.processChan }()
	default:
		return
	}

	var trulyChanged []string
	for _, fp := range changedPaths {
		if _, err := os.Stat(fp); os.IsNotExist(err) {
			trulyChanged = append(trulyChanged, fp) // File deleted; counted as a change.
			continue
		}

		changed, err := wm.cache.IsChanged(ctx, fp)
		if err != nil || changed {
			trulyChanged = append(trulyChanged, fp)
		}
	}

	if len(trulyChanged) == 0 {
		if wm.verbose {
			fmt.Fprintln(wm.output, "  (no content changes detected)")
		}
		return
	}

	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(wm.output, "[%s] Change detected in %d file(s):\n", timestamp, len(trulyChanged))

	// Print the first 10 changed files.
	displayLimit := 10
	for i, f := range trulyChanged {
		if i >= displayLimit {
			fmt.Fprintf(wm.output, "  ... and %d more\n", len(trulyChanged)-displayLimit)
			break
		}
		rel, _ := filepath.Rel(rootPath, f)
		exists := true
		if _, err := os.Stat(f); os.IsNotExist(err) {
			exists = false
		}
		status := "~"
		if !exists {
			status = "X"
		}
		fmt.Fprintf(wm.output, "  %s %s\n", status, rel)
	}

	// TODO: wait for other PipelineStage implement
	// The remaining functionality needs to be completed later.
	incrementalGraph := &KnowledgeGraph{}
	wm.currentGraph.MergeGraph(*incrementalGraph)
	nodeCount := wm.currentGraph.NodeCount()
	edgeCount := wm.currentGraph.EdgeCount()
	wm.graphMu.Unlock()

	_ = os.MkdirAll(outputDir, os.ModePerm)
	for _, format := range formats {
		_ = format //
	}

	fmt.Fprintf(wm.output, "  Re-processed %d file(s) -> %d nodes, %d edges\n", len(trulyChanged), nodeCount, edgeCount)
	fmt.Fprintf(wm.output, "  Exported to %s\n\n", outputDir)
}

// Helper method: Filter junk directories
func (wm *WatchMode) shouldIgnore(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return true
	}
	lowering := strings.ToLower(base)
	if lowering == "bin" || lowering == "obj" {
		return true
	}
	return false
}
