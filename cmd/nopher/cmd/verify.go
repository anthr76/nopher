package cmd

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/anthr76/nopher/internal/lockfile"
	"github.com/anthr76/nopher/internal/mod"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify [directory]",
	Short: "Verify lockfile matches go.mod/go.sum",
	Long: `Verify that the lockfile is in sync with go.mod and go.sum.

This command checks for:
- Missing modules in the lockfile
- Extra modules in the lockfile
- Version mismatches between lockfile and go.mod`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVerify,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Load existing lockfile
	lfPath := filepath.Join(dir, lockfile.DefaultLockfile)
	existing, err := lockfile.Load(lfPath)
	if err != nil {
		return fmt.Errorf("loading lockfile: %w", err)
	}

	// Parse go.mod
	goModPath := filepath.Join(dir, "go.mod")
	modInfo, err := mod.ParseGoMod(goModPath)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	// Check Go version
	if existing.Go != modInfo.GoVersion {
		return fmt.Errorf("Go version mismatch: lockfile has %s, go.mod has %s", existing.Go, modInfo.GoVersion)
	}

	// Build sets for comparison
	lockfileModules := make(map[string]string) // path -> version
	for path, m := range existing.Modules {
		lockfileModules[path] = m.Version
	}

	gomodModules := make(map[string]string)
	for _, req := range modInfo.Requires {
		gomodModules[req.Path] = req.Version
	}

	// Find differences
	var missing []string
	var extra []string
	var versionMismatch []string

	for path, version := range gomodModules {
		if lfVersion, ok := lockfileModules[path]; !ok {
			// Check if it's a local replace
			if rep, ok := existing.Replace[path]; ok && rep.Path != "" {
				continue // Local replace, skip
			}
			missing = append(missing, fmt.Sprintf("%s@%s", path, version))
		} else if lfVersion != version {
			versionMismatch = append(versionMismatch, fmt.Sprintf("%s: lockfile=%s, go.mod=%s", path, lfVersion, version))
		}
	}

	for path := range lockfileModules {
		if _, ok := gomodModules[path]; !ok {
			extra = append(extra, path)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(versionMismatch)

	if len(missing) > 0 || len(extra) > 0 || len(versionMismatch) > 0 {
		fmt.Println("Lockfile is out of sync with go.mod:")
		if len(missing) > 0 {
			fmt.Println("\nMissing from lockfile:")
			for _, m := range missing {
				fmt.Printf("  + %s\n", m)
			}
		}
		if len(extra) > 0 {
			fmt.Println("\nExtra in lockfile:")
			for _, m := range extra {
				fmt.Printf("  - %s\n", m)
			}
		}
		if len(versionMismatch) > 0 {
			fmt.Println("\nVersion mismatches:")
			for _, m := range versionMismatch {
				fmt.Printf("  ! %s\n", m)
			}
		}
		return fmt.Errorf("lockfile verification failed")
	}

	fmt.Println("Lockfile is in sync with go.mod")
	return nil
}
