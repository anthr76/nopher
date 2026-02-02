// nopher is a CLI tool for generating Nix-compatible lockfiles from Go modules.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/anthr76/nopher/internal/fetch"
	"github.com/anthr76/nopher/internal/lockfile"
	"github.com/anthr76/nopher/internal/mod"
)

const versionStr = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "generate":
		err = cmdGenerate(args)
	case "verify":
		err = cmdVerify(args)
	case "update":
		err = cmdUpdate(args)
	case "version", "-v", "--version":
		fmt.Printf("nopher version %s\n", versionStr)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `nopher - Generate Nix-compatible lockfiles from Go modules

Usage:
  nopher <command> [options]

Commands:
  generate    Generate lockfile from go.mod/go.sum
  verify      Verify lockfile matches go.mod/go.sum
  update      Update specific module in lockfile
  version     Print version information
  help        Show this help message

Run 'nopher <command> -h' for command-specific help.
`)
}

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output")
	tidy := fs.Bool("tidy", false, "run go mod tidy before generating (requires go)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: nopher generate [options] [directory]

Generate a lockfile from go.mod/go.sum.

Options:
`)
		fs.PrintDefaults()
	}
	fs.Parse(args)

	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	_ = tidy // TODO: implement tidy support

	// Parse go.mod
	goModPath := filepath.Join(dir, "go.mod")
	modInfo, err := mod.ParseGoMod(goModPath)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Module: %s\n", modInfo.ModulePath)
		fmt.Fprintf(os.Stderr, "Go version: %s\n", modInfo.GoVersion)
		fmt.Fprintf(os.Stderr, "Dependencies: %d\n", len(modInfo.Requires))
	}

	// Parse go.sum
	goSumPath := filepath.Join(dir, "go.sum")
	sumEntries, err := mod.ParseGoSum(goSumPath)
	if err != nil {
		return fmt.Errorf("parsing go.sum: %w", err)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Sum entries: %d\n", len(sumEntries))
	}

	// Create fetcher
	fetcher, err := fetch.NewFetcher()
	if err != nil {
		return fmt.Errorf("creating fetcher: %w", err)
	}
	fetcher.Verbose = *verbose

	// Create lockfile
	lf := lockfile.New(modInfo.GoVersion)

	// Build replace map for lookup
	replaceMap := make(map[string]mod.Replace)
	for _, rep := range modInfo.Replaces {
		replaceMap[rep.Old] = rep
	}

	// Process each required module
	for _, req := range modInfo.Requires {
		modulePath := req.Path
		moduleVersion := req.Version

		// Check if this module is replaced
		if rep, ok := replaceMap[modulePath]; ok {
			if rep.IsLocal {
				// Local replacement - add to replace section
				lf.Replace[modulePath] = lockfile.Replace{
					Path: rep.New,
				}
				if *verbose {
					fmt.Fprintf(os.Stderr, "  %s -> %s (local)\n", modulePath, rep.New)
				}
				continue
			}

			// Remote replacement
			if *verbose {
				fmt.Fprintf(os.Stderr, "  %s -> %s@%s (remote replace)\n", modulePath, rep.New, rep.NewVersion)
			}

			result, err := fetcher.Fetch(rep.New, rep.NewVersion)
			if err != nil {
				return fmt.Errorf("fetching replacement %s@%s: %w", rep.New, rep.NewVersion, err)
			}

			lf.Replace[modulePath] = lockfile.Replace{
				New:     rep.New,
				Version: rep.NewVersion,
				Hash:    result.Hash,
			}
			continue
		}

		// Regular dependency
		if *verbose {
			fmt.Fprintf(os.Stderr, "  %s@%s\n", modulePath, moduleVersion)
		}

		result, err := fetcher.Fetch(modulePath, moduleVersion)
		if err != nil {
			return fmt.Errorf("fetching %s@%s: %w", modulePath, moduleVersion, err)
		}

		lf.Modules[modulePath] = lockfile.Module{
			Version: moduleVersion,
			Hash:    result.Hash,
		}
	}

	// Save lockfile
	if err := lf.Save(dir); err != nil {
		return fmt.Errorf("saving lockfile: %w", err)
	}

	fmt.Printf("Generated %s\n", lockfile.DefaultLockfile)
	fmt.Printf("  Modules: %d\n", len(lf.Modules))
	fmt.Printf("  Replaces: %d\n", len(lf.Replace))

	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: nopher verify [directory]

Verify that the lockfile matches go.mod/go.sum.
`)
	}
	fs.Parse(args)

	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
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

func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: nopher update [options] <module-path> [directory]

Update a specific module in the lockfile.

Options:
`)
		fs.PrintDefaults()
	}
	fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("module path required")
	}

	modulePath := fs.Arg(0)
	dir := "."
	if fs.NArg() > 1 {
		dir = fs.Arg(1)
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
		if *verbose {
			fmt.Fprintf(os.Stderr, "Re-fetching %s@%s\n", modulePath, targetVersion)
		}
	} else if exists {
		if *verbose {
			fmt.Fprintf(os.Stderr, "Updating %s: %s -> %s\n", modulePath, current.Version, targetVersion)
		}
	} else {
		if *verbose {
			fmt.Fprintf(os.Stderr, "Adding %s@%s\n", modulePath, targetVersion)
		}
	}

	// Fetch the module
	fetcher, err := fetch.NewFetcher()
	if err != nil {
		return fmt.Errorf("creating fetcher: %w", err)
	}
	fetcher.Verbose = *verbose

	result, err := fetcher.Fetch(modulePath, targetVersion)
	if err != nil {
		return fmt.Errorf("fetching %s@%s: %w", modulePath, targetVersion, err)
	}

	// Update lockfile
	lf.Modules[modulePath] = lockfile.Module{
		Version: targetVersion,
		Hash:    result.Hash,
	}

	// Save
	if err := lf.Save(dir); err != nil {
		return fmt.Errorf("saving lockfile: %w", err)
	}

	fmt.Printf("Updated %s@%s\n", modulePath, targetVersion)
	fmt.Printf("  Hash: %s\n", trimHash(result.Hash))

	return nil
}

// trimHash shortens a hash for display.
func trimHash(hash string) string {
	if len(hash) > 40 {
		return hash[:40] + "..."
	}
	return hash
}
