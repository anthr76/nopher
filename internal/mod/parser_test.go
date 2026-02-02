package mod

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoMod(t *testing.T) {
	// Create temp directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		wantModule  string
		wantGo      string
		wantReqs    int
		wantReplace int
		wantErr     bool
	}{
		{
			name: "simple go.mod",
			content: `module github.com/example/repo

go 1.21

require (
	github.com/foo/bar v1.2.3
	github.com/baz/qux v0.1.0
)
`,
			wantModule:  "github.com/example/repo",
			wantGo:      "1.21",
			wantReqs:    2,
			wantReplace: 0,
		},
		{
			name: "with replacements",
			content: `module github.com/example/repo

go 1.21

require github.com/foo/bar v1.2.3

replace github.com/foo/bar => github.com/fork/bar v1.3.0
`,
			wantModule:  "github.com/example/repo",
			wantGo:      "1.21",
			wantReqs:    1,
			wantReplace: 1,
		},
		{
			name: "with local replacement",
			content: `module github.com/example/repo

go 1.21

require github.com/foo/bar v1.2.3

replace github.com/foo/bar => ./local/bar
`,
			wantModule:  "github.com/example/repo",
			wantGo:      "1.21",
			wantReqs:    1,
			wantReplace: 1,
		},
		{
			name: "with indirect dependencies",
			content: `module github.com/example/repo

go 1.21

require (
	github.com/foo/bar v1.2.3
	github.com/baz/qux v0.1.0 // indirect
)
`,
			wantModule:  "github.com/example/repo",
			wantGo:      "1.21",
			wantReqs:    2,
			wantReplace: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test go.mod
			goModPath := filepath.Join(tmpDir, "go.mod")
			if err := os.WriteFile(goModPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := ParseGoMod(goModPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGoMod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.ModulePath != tt.wantModule {
				t.Errorf("ModulePath = %q, want %q", got.ModulePath, tt.wantModule)
			}
			if got.GoVersion != tt.wantGo {
				t.Errorf("GoVersion = %q, want %q", got.GoVersion, tt.wantGo)
			}
			if len(got.Requires) != tt.wantReqs {
				t.Errorf("len(Requires) = %d, want %d", len(got.Requires), tt.wantReqs)
			}
			if len(got.Replaces) != tt.wantReplace {
				t.Errorf("len(Replaces) = %d, want %d", len(got.Replaces), tt.wantReplace)
			}
		})
	}
}

func TestParseGoSum(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    int
		wantErr bool
	}{
		{
			name: "simple go.sum",
			content: `github.com/foo/bar v1.2.3 h1:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=
github.com/foo/bar v1.2.3/go.mod h1:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=
`,
			want: 1, // Only counts module entries, not go.mod entries
		},
		{
			name:    "empty go.sum",
			content: "",
			want:    0,
		},
		{
			name: "comments and blank lines",
			content: `# comment
github.com/foo/bar v1.2.3 h1:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=

`,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goSumPath := filepath.Join(tmpDir, "go.sum")
			if err := os.WriteFile(goSumPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := ParseGoSum(goSumPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGoSum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(got) != tt.want {
				t.Errorf("len(entries) = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseReplaceDirective(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOld string
		wantNew string
		wantVer string
		isLocal bool
	}{
		{
			name:    "remote replacement",
			line:    "replace github.com/old/pkg => github.com/new/pkg v1.0.0",
			wantOld: "github.com/old/pkg",
			wantNew: "github.com/new/pkg",
			wantVer: "v1.0.0",
			isLocal: false,
		},
		{
			name:    "local replacement",
			line:    "replace github.com/old/pkg => ./local/path",
			wantOld: "github.com/old/pkg",
			wantNew: "./local/path",
			isLocal: true,
		},
		{
			name:    "replacement with version on left",
			line:    "replace github.com/old/pkg v1.0.0 => github.com/new/pkg v2.0.0",
			wantOld: "github.com/old/pkg",
			wantNew: "github.com/new/pkg",
			wantVer: "v2.0.0",
			isLocal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would require exposing parseReplace or testing indirectly through ParseGoMod
			// For now, we test through ParseGoMod
			tmpDir := t.TempDir()
			content := `module test

go 1.21

` + tt.line

			goModPath := filepath.Join(tmpDir, "go.mod")
			if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			info, err := ParseGoMod(goModPath)
			if err != nil {
				t.Fatal(err)
			}

			if len(info.Replaces) != 1 {
				t.Fatalf("expected 1 replace, got %d", len(info.Replaces))
			}

			rep := info.Replaces[0]
			if rep.Old != tt.wantOld {
				t.Errorf("Old = %q, want %q", rep.Old, tt.wantOld)
			}
			if rep.New != tt.wantNew {
				t.Errorf("New = %q, want %q", rep.New, tt.wantNew)
			}
			if !tt.isLocal && rep.NewVersion != tt.wantVer {
				t.Errorf("NewVersion = %q, want %q", rep.NewVersion, tt.wantVer)
			}
			if rep.IsLocal != tt.isLocal {
				t.Errorf("IsLocal = %v, want %v", rep.IsLocal, tt.isLocal)
			}
		})
	}
}
