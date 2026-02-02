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

var updateVerbose bool

var updateCmd = &cobra.Command{
	Use:   "update <module-path> [directory]",
	Short: "Update specific module in lockfile",
	Long: `Update a specific module in the lockfile to match go.mod.

This command re-fetches the module and updates its hash in the lockfile.
Useful for refreshing a single dependency without regenerating the entire lockfile.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVarP(&updateVerbose, "verbose", "v", false, "verbose output")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	modulePath := args[0]
	dir := "."
	if len(args) > 1 {
		dir = args[1]
	}

	// Load existing lockfile
	lfPath := filepath.Join(dir, lockfile.DefaultLockfile)
	lf, err := lockfile.Load(lfPath)
	if err != nil {
		return fmt.Errorf("loading lockfile: %w", err)
	}

	// Parse go.mod to get current version
	goModPath := filepath.Join(dir, "go.mod")
	modInfo, err := mod.ParseGoMod(goModPath)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	// Find the module
	var targetVersion string
	for _, req := range modInfo.Requires {
		if req.Path == modulePath {
			targetVersion = req.Version
			break
		}
	}

	if targetVersion == "" {
		return fmt.Errorf("module %s not found in go.mod", modulePath)
	}

	// Check current lockfile version
	current, exists := lf.Modules[modulePath]
	if exists && current.Version == targetVersion {
		if updateVerbose {
			fmt.Fprintf(os.Stderr, "Re-fetching %s@%s\n", modulePath, targetVersion)
		}
	} else if exists {
		if updateVerbose {
			fmt.Fprintf(os.Stderr, "Updating %s: %s -> %s\n", modulePath, current.Version, targetVersion)
		}
	} else {
		if updateVerbose {
			fmt.Fprintf(os.Stderr, "Adding %s@%s\n", modulePath, targetVersion)
		}
	}

	// Fetch the module
	fetcher, err := fetch.NewFetcher()
	if err != nil {
		return fmt.Errorf("creating fetcher: %w", err)
	}
	fetcher.Verbose = updateVerbose

	result, err := fetcher.Fetch(modulePath, targetVersion)
	if err != nil {
		return fmt.Errorf("fetching %s@%s: %w", modulePath, targetVersion, err)
	}

	// Update lockfile
	lf.Modules[modulePath] = lockfile.Module{
		Version: targetVersion,
		Hash:    result.Hash,
		URL:     result.URL,
		Rev:     result.Rev,
	}

	// Save
	if err := lf.Save(dir); err != nil {
		return fmt.Errorf("saving lockfile: %w", err)
	}

	fmt.Printf("Updated %s@%s\n", modulePath, targetVersion)
	fmt.Printf("  Hash: %s\n", trimHash(result.Hash))
	if updateVerbose && result.URL != "" {
		fmt.Printf("  URL: %s\n", result.URL)
	}

	return nil
}

func trimHash(hash string) string {
	if len(hash) > 40 {
		return hash[:40] + "..."
	}
	return hash
}
