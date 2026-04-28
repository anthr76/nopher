// Package generator exposes nopher lockfile generation as a library API.
package generator

import (
	"fmt"
	"path/filepath"

	"github.com/anthr76/nopher/internal/fetch"
	"github.com/anthr76/nopher/internal/mod"
	"github.com/anthr76/nopher/pkg/lockfile"
)

// FetchResult contains the lockfile-relevant metadata for a fetched module.
type FetchResult struct {
	Hash string
	URL  string
	Rev  string
}

// FetchFunc fetches metadata for a single module version.
type FetchFunc func(modulePath, version string) (*FetchResult, error)

// Options configures lockfile generation.
type Options struct {
	// Verbose enables verbose output from the default fetcher.
	Verbose bool
	// Fetch overrides module fetching. When nil, generator uses nopher's default fetcher.
	Fetch FetchFunc
}

// Generate creates a lockfile from go.mod and go.sum in dir without writing it.
func Generate(dir string, opts Options) (*lockfile.Lockfile, error) {
	if dir == "" {
		dir = "."
	}

	goModPath := filepath.Join(dir, "go.mod")
	modInfo, err := mod.ParseGoMod(goModPath)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}

	goSumPath := filepath.Join(dir, "go.sum")
	sumEntriesList, err := mod.ParseGoSum(goSumPath)
	if err != nil {
		return nil, fmt.Errorf("parsing go.sum: %w", err)
	}

	sumEntries := make(map[string]bool)
	for _, entry := range sumEntriesList {
		sumEntries[moduleKey(entry.Path, entry.Version)] = true
	}

	goModOnlyEntries, err := mod.ParseGoSumModOnly(goSumPath)
	if err != nil {
		return nil, fmt.Errorf("parsing go.sum for go.mod entries: %w", err)
	}
	for _, entry := range goModOnlyEntries {
		sumEntries[moduleKey(entry.Path, entry.Version)] = true
	}

	fetchModule, err := fetchFunc(opts)
	if err != nil {
		return nil, err
	}

	lf := lockfile.New(modInfo.GoVersion)

	requireMap := make(map[string]string)
	for _, req := range modInfo.Requires {
		requireMap[req.Path] = req.Version
	}

	for _, rep := range modInfo.Replaces {
		if rep.IsLocal {
			lf.Replace[rep.Old] = lockfile.Replace{
				Path: rep.New,
			}
			continue
		}

		result, err := fetchModule(rep.New, rep.NewVersion)
		if err != nil {
			return nil, fmt.Errorf("fetching replacement %s@%s: %w", rep.New, rep.NewVersion, err)
		}
		if result == nil {
			return nil, fmt.Errorf("fetching replacement %s@%s: no result", rep.New, rep.NewVersion)
		}

		oldVersion := rep.OldVersion
		if oldVersion == "" {
			oldVersion = requireMap[rep.Old]
		}

		lf.Replace[rep.Old] = lockfile.Replace{
			Old:        rep.Old,
			OldVersion: oldVersion,
			New:        rep.New,
			Version:    rep.NewVersion,
			Hash:       result.Hash,
			URL:        result.URL,
			Rev:        result.Rev,
		}
	}

	for _, req := range modInfo.Requires {
		modulePath := req.Path
		moduleVersion := req.Version

		if _, ok := lf.Replace[modulePath]; ok {
			continue
		}

		if _, ok := sumEntries[moduleKey(modulePath, moduleVersion)]; !ok {
			continue
		}

		result, err := fetchModule(modulePath, moduleVersion)
		if err != nil {
			return nil, fmt.Errorf("fetching %s@%s: %w", modulePath, moduleVersion, err)
		}
		if result == nil {
			return nil, fmt.Errorf("fetching %s@%s: no result", modulePath, moduleVersion)
		}

		lf.Modules[modulePath] = lockfile.Module{
			Version: moduleVersion,
			Hash:    result.Hash,
			URL:     result.URL,
			Rev:     result.Rev,
		}
	}

	return lf, nil
}

// GenerateAndSave creates a lockfile from go.mod and go.sum in dir and writes it
// to nopher.lock.yaml.
func GenerateAndSave(dir string, opts Options) (*lockfile.Lockfile, error) {
	lf, err := Generate(dir, opts)
	if err != nil {
		return nil, err
	}

	if dir == "" {
		dir = "."
	}
	if err := lf.Save(dir); err != nil {
		return nil, fmt.Errorf("saving lockfile: %w", err)
	}

	return lf, nil
}

func fetchFunc(opts Options) (FetchFunc, error) {
	if opts.Fetch != nil {
		return opts.Fetch, nil
	}

	fetcher, err := fetch.NewFetcher()
	if err != nil {
		return nil, fmt.Errorf("creating fetcher: %w", err)
	}
	fetcher.Verbose = opts.Verbose

	return func(modulePath, version string) (*FetchResult, error) {
		result, err := fetcher.Fetch(modulePath, version)
		if err != nil {
			return nil, err
		}

		return &FetchResult{
			Hash: result.Hash,
			URL:  result.URL,
			Rev:  result.Rev,
		}, nil
	}, nil
}

func moduleKey(path, version string) string {
	return path + "@" + version
}
