package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetState clears the package-level cache and promptDir so tests are isolated.
func resetState(t *testing.T) {
	t.Helper()
	cacheMu.Lock()
	cache = make(map[string]string)
	promptDir = ""
	cacheMu.Unlock()
}

func TestInit_ValidDir(t *testing.T) {
	dir := t.TempDir()

	// Write a couple of .md files.
	if err := os.WriteFile(filepath.Join(dir, "agent.md"), []byte("# Agent prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "planner.md"), []byte("# Planner prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { resetState(t) })

	if err := Init(dir); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	cacheMu.RLock()
	count := len(cache)
	cacheMu.RUnlock()

	if count != 2 {
		t.Errorf("expected 2 cached prompts, got %d", count)
	}
}

func TestInit_InvalidPath(t *testing.T) {
	t.Cleanup(func() { resetState(t) })

	// A path that cannot be read (does not exist).
	err := Init("/this/path/definitely/does/not/exist/abc123")
	// Reload returns nil even when the directory is missing.
	if err != nil {
		t.Errorf("Init should return nil on missing dir, got: %v", err)
	}
}

func TestGet_CachedPrompt(t *testing.T) {
	dir := t.TempDir()
	content := "You are a helpful analyst."
	if err := os.WriteFile(filepath.Join(dir, "analyst.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { resetState(t) })

	if err := Init(dir); err != nil {
		t.Fatal(err)
	}

	got := Get("analyst.md")
	if got != content {
		t.Errorf("Get(\"analyst.md\") = %q, want %q", got, content)
	}
}

func TestGet_MissingPrompt(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { resetState(t) })

	if err := Init(dir); err != nil {
		t.Fatal(err)
	}

	got := Get("nonexistent.md")
	if !strings.HasPrefix(got, "CRITICAL ERROR:") {
		t.Errorf("Get of missing prompt should return CRITICAL ERROR, got: %q", got)
	}
	if !strings.Contains(got, "nonexistent.md") {
		t.Errorf("error message should include filename; got: %q", got)
	}
}

func TestGet_LazyLoad(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { resetState(t) })

	// Init on empty dir — no prompts loaded.
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}

	// Write a file AFTER Init.
	content := "lazy loaded content"
	if err := os.WriteFile(filepath.Join(dir, "lazy.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := Get("lazy.md")
	if got != content {
		t.Errorf("Get lazy load = %q, want %q", got, content)
	}
}

func TestReload_ClearsAndReloadsCache(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "watchdog.md")
	t.Cleanup(func() { resetState(t) })

	// Initial content.
	if err := os.WriteFile(file, []byte("version one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if got := Get("watchdog.md"); got != "version one" {
		t.Fatalf("initial load = %q, want %q", got, "version one")
	}

	// Update the file.
	if err := os.WriteFile(file, []byte("version two"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Reload(); err != nil {
		t.Fatal(err)
	}

	got := Get("watchdog.md")
	if got != "version two" {
		t.Errorf("after Reload, Get = %q, want %q", got, "version two")
	}
}

func TestInit_SkipsNonMdFiles(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { resetState(t) })

	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text file"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "real.md"), []byte("md file"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Init(dir); err != nil {
		t.Fatal(err)
	}

	cacheMu.RLock()
	_, hasTxt := cache["notes.txt"]
	_, hasMd := cache["real.md"]
	cacheMu.RUnlock()

	if hasTxt {
		t.Error(".txt file should not be loaded into cache")
	}
	if !hasMd {
		t.Error(".md file should be loaded into cache")
	}
}

func TestInit_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { resetState(t) })

	// Create a subdirectory — Init must not recurse into it or panic.
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := Init(dir); err != nil {
		t.Fatalf("Init should not error on directory with subdirectory: %v", err)
	}
}

func TestGet_CachedAfterLazyLoad(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { resetState(t) })

	if err := Init(dir); err != nil {
		t.Fatal(err)
	}

	// Write a file AFTER Init and call Get twice — second call must hit cache.
	content := "cached content"
	if err := os.WriteFile(filepath.Join(dir, "twice.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	first := Get("twice.md")
	second := Get("twice.md")

	if first != content || second != content {
		t.Errorf("expected %q for both calls, got %q and %q", content, first, second)
	}

	cacheMu.RLock()
	cached, ok := cache["twice.md"]
	cacheMu.RUnlock()
	if !ok || cached != content {
		t.Error("lazy-loaded prompt should be stored in cache")
	}
}

func TestReload_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { resetState(t) })

	if err := Init(dir); err != nil {
		t.Fatal(err)
	}

	// Reload on an empty directory must succeed and leave cache empty.
	if err := Reload(); err != nil {
		t.Fatalf("Reload on empty dir returned error: %v", err)
	}

	cacheMu.RLock()
	count := len(cache)
	cacheMu.RUnlock()

	if count != 0 {
		t.Errorf("expected empty cache, got %d entries", count)
	}
}
