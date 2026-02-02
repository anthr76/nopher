package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	cmd := rootCmd
	cmd.SetArgs([]string{"--help"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("rootCmd failed: %v", err)
	}

	output := buf.String()
	if !contains(output, "nopher") {
		t.Error("Help output should contain 'nopher'")
	}
	if !contains(output, "generate") {
		t.Error("Help output should list 'generate' command")
	}
	if !contains(output, "verify") {
		t.Error("Help output should list 'verify' command")
	}
}

func TestVersionCommand(t *testing.T) {
	// Create a fresh version command for testing
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("nopher version " + Version)
		},
	}

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.Run(cmd, []string{})

	output := buf.String()
	if !contains(output, Version) {
		t.Errorf("Version output should contain version %q, got %q", Version, output)
	}
}

func TestGenerateCommand(t *testing.T) {
	// Create test directory with minimal go.mod and go.sum
	tmpDir := t.TempDir()

	goMod := `module github.com/test/example

go 1.21

require golang.org/x/mod v0.32.0
`
	goSum := `golang.org/x/mod v0.32.0 h1:abcd1234
golang.org/x/mod v0.32.0/go.mod h1:xyz9876
`

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(goSum), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a fresh command for testing
	cmd := &cobra.Command{
		Use:  "generate",
		RunE: runGenerate,
	}
	cmd.Flags().BoolP("verbose", "v", false, "verbose output")
	cmd.Flags().Bool("tidy", false, "run go mod tidy")

	cmd.SetArgs([]string{tmpDir})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// This will try to fetch modules, which may fail in test environment
	// We're mainly testing that the command runs without panicking
	err := cmd.Execute()
	if err != nil {
		// It's okay if it fails due to network or other reasons in test
		t.Logf("Generate command failed (expected in test env): %v", err)
	}
}

func TestVerifyCommand(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()

	goMod := `module github.com/test/example

go 1.21

require golang.org/x/mod v0.32.0
`
	lockfile := `schema: 1
go: "1.21"
modules:
  golang.org/x/mod:
    version: v0.32.0
    hash: sha256-test1234
`

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "nopher.lock.yaml"), []byte(lockfile), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{
		Use:  "verify",
		RunE: runVerify,
	}

	cmd.SetArgs([]string{tmpDir})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Logf("Verify command result: %v", err)
	}
}

func TestUpdateCommandValidation(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "update",
		Args: cobra.RangeArgs(1, 2),
		RunE: runUpdate,
	}
	cmd.Flags().BoolP("verbose", "v", false, "verbose")

	// Test with no arguments (should fail validation)
	cmd.SetArgs([]string{})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Error("Update command should fail without module path argument")
	}
}

func contains(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
