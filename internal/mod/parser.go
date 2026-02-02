// Package mod provides functionality for parsing go.mod and go.sum files.
package mod

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// ModInfo contains parsed information from go.mod.
type ModInfo struct {
	ModulePath string
	GoVersion  string
	Requires   []Require
	Replaces   []Replace
}

// Require represents a single require directive.
type Require struct {
	Path     string
	Version  string
	Indirect bool
}

// Replace represents a replace directive.
type Replace struct {
	Old        string
	OldVersion string
	New        string
	NewVersion string
	IsLocal    bool // True if New is a local filesystem path
}

// SumEntry represents a single entry from go.sum.
type SumEntry struct {
	Path    string
	Version string
	Hash    string // The h1: hash
}

// ParseGoMod reads and parses a go.mod file.
func ParseGoMod(path string) (*ModInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading go.mod: %w", err)
	}

	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}

	info := &ModInfo{
		ModulePath: f.Module.Mod.Path,
	}

	if f.Go != nil {
		info.GoVersion = f.Go.Version
	}

	for _, req := range f.Require {
		info.Requires = append(info.Requires, Require{
			Path:     req.Mod.Path,
			Version:  req.Mod.Version,
			Indirect: req.Indirect,
		})
	}

	for _, rep := range f.Replace {
		r := Replace{
			Old:        rep.Old.Path,
			OldVersion: rep.Old.Version,
			New:        rep.New.Path,
			NewVersion: rep.New.Version,
		}
		// Check if it's a local path replacement
		if strings.HasPrefix(rep.New.Path, "./") || strings.HasPrefix(rep.New.Path, "../") || filepath.IsAbs(rep.New.Path) {
			r.IsLocal = true
		}
		info.Replaces = append(info.Replaces, r)
	}

	return info, nil
}

// ParseGoSum reads and parses a go.sum file.
func ParseGoSum(path string) ([]SumEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening go.sum: %w", err)
	}
	defer f.Close()

	var entries []SumEntry
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 3 {
			continue // Skip malformed lines
		}

		modulePath := parts[0]
		version := parts[1]
		hash := parts[2]

		// Skip /go.mod entries, we only want the module zip hashes
		if strings.HasSuffix(version, "/go.mod") {
			continue
		}

		entries = append(entries, SumEntry{
			Path:    modulePath,
			Version: version,
			Hash:    hash,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning go.sum: %w", err)
	}

	return entries, nil
}

// SumMap converts a slice of SumEntry to a map keyed by path@version.
func SumMap(entries []SumEntry) map[string]string {
	m := make(map[string]string)
	for _, e := range entries {
		key := e.Path + "@" + e.Version
		m[key] = e.Hash
	}
	return m
}
