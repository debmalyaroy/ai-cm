package prompts

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	cache     = make(map[string]string)
	cacheMu   sync.RWMutex
	promptDir string
)

// Init configures the base directory where markdown prompts live.
// It triggers an initial load of all files in the directory.
func Init(dir string) error {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for prompts: %w", err)
	}

	promptDir = absPath
	return Reload()
}

// Reload clears the cache and loads all fresh prompts from disk.
// This allows hot-swapping behavior without a full app restart.
func Reload() error {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Clear existing cache
	cache = make(map[string]string)

	entries, err := os.ReadDir(promptDir)
	if err != nil {
		slog.Warn("Prompts directory missing or inaccessible. Prompts may fail.", "dir", promptDir, "err", err)
		return nil // Return nil so application doesn't completely crash if testing without a prompt dir
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		path := filepath.Join(promptDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Error("Failed to read prompt file", "file", path, "err", err)
			continue
		}

		cache[entry.Name()] = string(data)
		loaded++
	}

	slog.Info("Successfully loaded system prompts into memory", "count", loaded, "dir", promptDir)
	return nil
}

// Get retrieves a prompt by its filename (e.g., "planner.md").
// If the prompt isn't found in cache, it will attempt a lazy-load from disk
// in case a single new file was dropped into the directory.
func Get(filename string) string {
	cacheMu.RLock()
	val, ok := cache[filename]
	cacheMu.RUnlock()

	if ok {
		return val
	}

	// Lazy load attempt
	path := filepath.Join(promptDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("CRITICAL: Prompt not found in cache or disk", "filename", filename, "path", path)
		return fmt.Sprintf("CRITICAL ERROR: MISSING PROMPT '%s'", filename)
	}

	promptStr := string(data)

	// Save back to cache
	cacheMu.Lock()
	cache[filename] = promptStr
	cacheMu.Unlock()

	slog.Debug("Lazy-loaded missing prompt from disk", "filename", filename)
	return promptStr
}
