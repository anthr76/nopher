package cmd

import (
	"fmt"

	"github.com/anthr76/nopher/pkg/generator"
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

	lf, err := generator.GenerateAndSave(dir, generator.Options{
		Verbose: generateVerbose,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Generated lockfile with %d modules\n", len(lf.Modules))
	if len(lf.Replace) > 0 {
		fmt.Printf("  Replacements: %d\n", len(lf.Replace))
	}

	return nil
}
