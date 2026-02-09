package mod

import (
	"os"
	"testing"
)

func TestReplaceOldVersionLookup(t *testing.T) {
	goModContent := `module example.com/test

go 1.25

require (
	github.com/old/pkg v1.2.3
)

replace github.com/old/pkg => github.com/new/pkg v2.0.0
`
	tmpfile := t.TempDir() + "/go.mod"
	if err := os.WriteFile(tmpfile, []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := ParseGoMod(tmpfile)
	if err != nil {
		t.Fatalf("ParseGoMod() error = %v", err)
	}

	if len(info.Replaces) != 1 {
		t.Fatalf("Expected 1 replace, got %d", len(info.Replaces))
	}

	rep := info.Replaces[0]

	if rep.Old != "github.com/old/pkg" {
		t.Errorf("Old = %v, want github.com/old/pkg", rep.Old)
	}

	if rep.OldVersion != "" {
		t.Errorf("OldVersion = %v, want empty (parser doesn't populate this, generate.go does the lookup)", rep.OldVersion)
	}

	if rep.New != "github.com/new/pkg" {
		t.Errorf("New = %v, want github.com/new/pkg", rep.New)
	}

	if rep.NewVersion != "v2.0.0" {
		t.Errorf("NewVersion = %v, want v2.0.0", rep.NewVersion)
	}
}
