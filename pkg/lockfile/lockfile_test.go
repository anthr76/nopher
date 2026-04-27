package lockfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	goVersion := "1.21"
	lf := New(goVersion)

	if lf.Schema != SchemaVersion {
		t.Errorf("Schema = %d, want %d", lf.Schema, SchemaVersion)
	}
	if lf.Go != goVersion {
		t.Errorf("Go = %q, want %q", lf.Go, goVersion)
	}
	if lf.Modules == nil {
		t.Error("Modules is nil, want initialized map")
	}
	if lf.Replace == nil {
		t.Error("Replace is nil, want initialized map")
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a lockfile
	original := &Lockfile{
		Schema: 1,
		Go:     "1.21",
		Modules: map[string]Module{
			"github.com/example/repo": {
				Version: "v1.2.3",
				Hash:    "sha256-abcd1234",
				URL:     "https://github.com/example/repo/archive/refs/tags/v1.2.3.zip",
				Rev:     "abc123def456",
			},
		},
		Replace: map[string]Replace{
			"github.com/old/pkg": {
				New:     "github.com/new/pkg",
				Version: "v2.0.0",
				Hash:    "sha256-xyz9876",
			},
		},
	}

	// Save it
	if err := original.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load it back
	lfPath := filepath.Join(tmpDir, DefaultLockfile)
	loaded, err := Load(lfPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Compare
	if loaded.Schema != original.Schema {
		t.Errorf("Schema = %d, want %d", loaded.Schema, original.Schema)
	}
	if loaded.Go != original.Go {
		t.Errorf("Go = %q, want %q", loaded.Go, original.Go)
	}
	if len(loaded.Modules) != len(original.Modules) {
		t.Errorf("len(Modules) = %d, want %d", len(loaded.Modules), len(original.Modules))
	}

	// Check module details
	if mod, ok := loaded.Modules["github.com/example/repo"]; ok {
		origMod := original.Modules["github.com/example/repo"]
		if mod.Version != origMod.Version {
			t.Errorf("Module.Version = %q, want %q", mod.Version, origMod.Version)
		}
		if mod.Hash != origMod.Hash {
			t.Errorf("Module.Hash = %q, want %q", mod.Hash, origMod.Hash)
		}
		if mod.URL != origMod.URL {
			t.Errorf("Module.URL = %q, want %q", mod.URL, origMod.URL)
		}
		if mod.Rev != origMod.Rev {
			t.Errorf("Module.Rev = %q, want %q", mod.Rev, origMod.Rev)
		}
	} else {
		t.Error("Module not found in loaded lockfile")
	}

	// Check replace details
	if rep, ok := loaded.Replace["github.com/old/pkg"]; ok {
		origRep := original.Replace["github.com/old/pkg"]
		if rep.New != origRep.New {
			t.Errorf("Replace.New = %q, want %q", rep.New, origRep.New)
		}
		if rep.Version != origRep.Version {
			t.Errorf("Replace.Version = %q, want %q", rep.Version, origRep.Version)
		}
	} else {
		t.Error("Replace not found in loaded lockfile")
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/to/lockfile.yaml")
	if err == nil {
		t.Error("Load() on nonexistent file should return error")
	}
}

func TestSaveInvalidDirectory(t *testing.T) {
	lf := New("1.21")
	err := lf.Save("/nonexistent/invalid/directory")
	if err == nil {
		t.Error("Save() to invalid directory should return error")
	}
}

func TestLocalReplacement(t *testing.T) {
	tmpDir := t.TempDir()

	lf := &Lockfile{
		Schema:  1,
		Go:      "1.21",
		Modules: map[string]Module{},
		Replace: map[string]Replace{
			"github.com/example/repo": {
				Path: "./local/path",
			},
		},
	}

	// Save and load
	if err := lf.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	lfPath := filepath.Join(tmpDir, DefaultLockfile)
	loaded, err := Load(lfPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rep, ok := loaded.Replace["github.com/example/repo"]
	if !ok {
		t.Fatal("Replace not found")
	}
	if rep.Path != "./local/path" {
		t.Errorf("Replace.Path = %q, want %q", rep.Path, "./local/path")
	}
	if rep.New != "" {
		t.Errorf("Replace.New = %q, want empty", rep.New)
	}
	if rep.Hash != "" {
		t.Errorf("Replace.Hash = %q, want empty", rep.Hash)
	}
}

func TestYAMLOmitEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	lf := &Lockfile{
		Schema: 1,
		Go:     "1.21",
		Modules: map[string]Module{
			"github.com/example/repo": {
				Version: "v1.2.3",
				Hash:    "sha256-abcd1234",
				// URL and Rev omitted
			},
		},
		Replace: map[string]Replace{},
	}

	if err := lf.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read raw YAML
	lfPath := filepath.Join(tmpDir, DefaultLockfile)
	content, err := os.ReadFile(lfPath)
	if err != nil {
		t.Fatal(err)
	}

	yamlStr := string(content)

	// URL and Rev should not appear in YAML when empty (omitempty)
	if containsString(yamlStr, "url:") {
		t.Error("YAML contains 'url:' field, should be omitted when empty")
	}
	if containsString(yamlStr, "rev:") {
		t.Error("YAML contains 'rev:' field, should be omitted when empty")
	}

	// But version and hash should be present
	if !containsString(yamlStr, "version:") {
		t.Error("YAML missing 'version:' field")
	}
	if !containsString(yamlStr, "hash:") {
		t.Error("YAML missing 'hash:' field")
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
