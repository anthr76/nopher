package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const Version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "nopher",
	Short: "Generate Nix-compatible lockfiles from Go modules",
	Long: `nopher generates Nix-compatible lockfiles from Go module dependencies.

It parses go.mod and go.sum to create a nopher.lock.yaml file that can be
used by Nix's buildNopherGoApp to build Go applications reproducibly.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
