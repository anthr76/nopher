package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthr76/nopher/internal/fetch"
	"github.com/anthr76/nopher/internal/lockfile"
	"github.com/anthr76/nopher/internal/mod"
	"github.com/spf13/cobra"
)

var (
	generateVerbose bool
	generateTidy    bool
)

var generateCmd = &cobra.Command{
	Use:   "generate [directory]",
	Short: "Generate lockfile from go.mod/go.sum",
	Long: `Generate a nopher.lock.yaml file from go.mod and go.sum.

The lockfile contains all module dependencies with their versions and hashes,
enabling reproducible Nix builds.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().BoolVarP(&generateVerbose, "verbose", "v", false, "verbose output")
	generateCmd.Flags().BoolVar(&generateTidy, "tidy", false, "run go mod tidy before generating (requires go)")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	_ = generateTidy // TODO: implement tidy support

	// Parse go.mod
	goModPath := filepath.Join(dir, "go.mod")
	modInfo, err := mod.ParseGoMod(goModPath)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	if generateVerbose {
		fmt.Fprintf(os.Stderr, "Module: %s\n", modInfo.ModulePath)
		fmt.Fprintf(os.Stderr, "Go version: %s\n", modInfo.GoVersion)
		fmt.Fprintf(os.Stderr, "Dependencies: %d\n", len(modInfo.Requires))
		if len(modInfo.Replaces) > 0 {
			fmt.Fprintf(os.Stderr, "Replacements: %d\n", len(modInfo.Replaces))
		}
	}

	// Parse go.sum
	goSumPath := filepath.Join(dir, "go.sum")
	sumEntriesList, err := mod.ParseGoSum(goSumPath)
	if err != nil {
		return fmt.Errorf("parsing go.sum: %w", err)
	}

	// Build map for fast lookup
	sumEntries := make(map[string]bool)
	for _, entry := range sumEntriesList {
		key := entry.Path + "@" + entry.Version
		sumEntries[key] = true
	}

	if generateVerbose {
		fmt.Fprintf(os.Stderr, "Entries in go.sum: %d\n", len(sumEntries))
	}

	// Build lockfile
	lf := &lockfile.Lockfile{
		Schema:  lockfile.SchemaVersion,
		Go:      modInfo.GoVersion,
		Modules: make(map[string]lockfile.Module),
		Replace: make(map[string]lockfile.Replace),
	}

	// Create fetcher
	fetcher, err := fetch.NewFetcher()
	if err != nil {
		return fmt.Errorf("creating fetcher: %w", err)
	}
	fetcher.Verbose = generateVerbose

	// Process replacements first
	for _, rep := range modInfo.Replaces {
		if rep.IsLocal {
			// Local replacement
			lf.Replace[rep.Old] = lockfile.Replace{
				Path: rep.New,
			}
			if generateVerbose {
				fmt.Fprintf(os.Stderr, "Local replace: %s -> %s\n", rep.Old, rep.New)
			}
		} else {
			// Remote replacement
			moduleVersion := rep.NewVersion
			if generateVerbose {
				fmt.Fprintf(os.Stderr, "Fetching replacement: %s@%s\n", rep.New, moduleVersion)
			}

			result, err := fetcher.Fetch(rep.New, moduleVersion)
			if err != nil {
				return fmt.Errorf("fetching replacement %s@%s: %w", rep.New, moduleVersion, err)
			}

			lf.Replace[rep.Old] = lockfile.Replace{
				Old:        rep.Old,
				OldVersion: rep.OldVersion,
				New:        rep.New,
				Version:    rep.NewVersion,
				Hash:       result.Hash,
				URL:        result.URL,
				Rev:        result.Rev,
			}
			continue
		}
	}

	// Fetch each module
	for _, req := range modInfo.Requires {
		modulePath := req.Path
		moduleVersion := req.Version

		// Skip if it's locally replaced
		if rep, ok := lf.Replace[modulePath]; ok && rep.Path != "" {
			continue
		}

		// Check if module is in go.sum
		key := modulePath + "@" + moduleVersion
		if _, ok := sumEntries[key]; !ok {
			if generateVerbose {
				fmt.Fprintf(os.Stderr, "Skipping %s@%s (not in go.sum)\n", modulePath, moduleVersion)
			}
			continue
		}

		if generateVerbose {
			fmt.Fprintf(os.Stderr, "Fetching: %s@%s\n", modulePath, moduleVersion)
		}

		result, err := fetcher.Fetch(modulePath, moduleVersion)
		if err != nil {
			return fmt.Errorf("fetching %s@%s: %w", modulePath, moduleVersion, err)
		}

		lf.Modules[modulePath] = lockfile.Module{
			Version: moduleVersion,
			Hash:    result.Hash,
			URL:     result.URL,
			Rev:     result.Rev,
		}
	}

	// Save lockfile
	if err := lf.Save(dir); err != nil {
		return fmt.Errorf("saving lockfile: %w", err)
	}

	fmt.Printf("Generated lockfile with %d modules\n", len(lf.Modules))
	if len(lf.Replace) > 0 {
		fmt.Printf("  Replacements: %d\n", len(lf.Replace))
	}

	return nil
}
