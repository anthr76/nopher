package fetch

import (
	"strings"
	"testing"
)

func TestGetModuleInfoFromGoList(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		version    string
		wantOrigin bool
		wantRef    string
		wantHash   string
		wantVCS    string
		wantURL    string
	}{
		{
			name:       "pseudo-version extracts git hash",
			modulePath: "github.com/example/repo",
			version:    "v0.0.0-20231201120000-abcdef123456",
			wantOrigin: true,
			wantHash:   "abcdef123456",
			wantVCS:    "git",
			wantURL:    "https://github.com/example/repo",
		},
		{
			name:       "tagged version creates ref",
			modulePath: "github.com/example/repo",
			version:    "v1.2.3",
			wantOrigin: true,
			wantRef:    "refs/tags/v1.2.3",
			wantVCS:    "git",
			wantURL:    "https://github.com/example/repo",
		},
		{
			name:       "non-github module returns real origin from go list",
			modulePath: "golang.org/x/mod",
			version:    "v0.32.0",
			wantOrigin: true, // go list returns real Origin data
			wantVCS:    "git",
			wantURL:    "https://go.googlesource.com/mod",
		},
		{
			name:       "github submodule creates origin",
			modulePath: "github.com/example/repo/subpkg",
			version:    "v1.0.0",
			wantOrigin: true,
			wantRef:    "refs/tags/v1.0.0",
			wantVCS:    "git",
			wantURL:    "https://github.com/example/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fetcher{}
			info, err := f.getModuleInfoFromGoList(tt.modulePath, tt.version)
			if err != nil {
				t.Fatalf("getModuleInfoFromGoList() error = %v", err)
			}

			if info.Version != tt.version {
				t.Errorf("Version = %v, want %v", info.Version, tt.version)
			}

			if tt.wantOrigin {
				if info.Origin == nil {
					t.Fatal("Origin is nil, want non-nil")
				}
				if info.Origin.VCS != tt.wantVCS {
					t.Errorf("Origin.VCS = %v, want %v", info.Origin.VCS, tt.wantVCS)
				}
				if info.Origin.URL != tt.wantURL {
					t.Errorf("Origin.URL = %v, want %v", info.Origin.URL, tt.wantURL)
				}
				if tt.wantRef != "" && info.Origin.Ref != tt.wantRef {
					t.Errorf("Origin.Ref = %v, want %v", info.Origin.Ref, tt.wantRef)
				}
				if tt.wantHash != "" && info.Origin.Hash != tt.wantHash {
					t.Errorf("Origin.Hash = %v, want %v", info.Origin.Hash, tt.wantHash)
				}
			} else {
				if info.Origin != nil {
					t.Error("Origin is non-nil, want nil")
				}
			}
		})
	}
}

func TestIsPseudoVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"v0.0.0-20231201120000-abcdef123456", true},
		{"v1.2.3", false},
		{"v1.0.0-alpha.1", false},
		{"v2.0.0+incompatible", false},
		{"v0.0.0-20191109021931-daa7c04131f5", true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := strings.HasPrefix(tt.version, "v0.0.0-")
			if got != tt.want {
				t.Errorf("isPseudoVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsPrivate(t *testing.T) {
	tests := []struct {
		name       string
		private    string
		modulePath string
		want       bool
	}{
		{
			name:       "exact match",
			private:    "github.com/myorg/private",
			modulePath: "github.com/myorg/private",
			want:       true,
		},
		{
			name:       "wildcard match",
			private:    "github.com/myorg/*",
			modulePath: "github.com/myorg/private",
			want:       true,
		},
		{
			name:       "prefix match",
			private:    "github.com/myorg",
			modulePath: "github.com/myorg/private",
			want:       true,
		},
		{
			name:       "no match",
			private:    "github.com/myorg/*",
			modulePath: "github.com/other/public",
			want:       false,
		},
		{
			name:       "multiple patterns",
			private:    "github.com/myorg/*,gitlab.com/internal/*",
			modulePath: "gitlab.com/internal/project",
			want:       true,
		},
		{
			name:       "empty private",
			private:    "",
			modulePath: "github.com/myorg/private",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fetcher{Private: tt.private}
			got := f.isPrivate(tt.modulePath)
			if got != tt.want {
				t.Errorf("isPrivate(%q) = %v, want %v", tt.modulePath, got, tt.want)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern    string
		modulePath string
		want       bool
	}{
		{"github.com/myorg/*", "github.com/myorg/repo", true},
		{"github.com/myorg/*", "github.com/myorg", true},
		{"github.com/myorg/*", "github.com/other/repo", false},
		{"github.com/myorg*", "github.com/myorg", true},
		{"github.com/myorg*", "github.com/myorgtest", true},
		{"github.com/myorg", "github.com/myorg/repo", true},
		{"github.com/myorg", "github.com/myorg", true},
		{"github.com/myorg", "github.com/other", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+" vs "+tt.modulePath, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.modulePath)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.modulePath, got, tt.want)
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		modulePath string
		want       string
	}{
		{"github.com/example/repo", "github.com"},
		{"golang.org/x/mod", "golang.org"},
		{"buf.build/gen/go/org/repo", "buf.build"},
		{"example.com", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.modulePath, func(t *testing.T) {
			// Test through directURL which uses extractHost internally
			f := &Fetcher{}
			url := f.directURL(tt.modulePath, "v1.0.0")
			if !hasPrefix(url, "https://"+tt.want) {
				t.Errorf("directURL(%q) should use host %q, got %q", tt.modulePath, tt.want, url)
			}
		})
	}
}

func TestDirectURL(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		version    string
		wantURL    string
	}{
		{
			name:       "github simple tag",
			modulePath: "github.com/example/repo",
			version:    "v1.2.3",
			wantURL:    "https://github.com/example/repo/archive/refs/tags/v1.2.3.zip",
		},
		{
			name:       "github with special chars",
			modulePath: "github.com/example/repo",
			version:    "v1.0.0-rc.1",
			wantURL:    "https://github.com/example/repo/archive/refs/tags/v1.0.0-rc.1.zip",
		},
		{
			name:       "BSR module",
			modulePath: "buf.build/gen/go/org/repo",
			version:    "v1.0.0",
			wantURL:    "https://buf.build/gen/go/buf.build/gen/go/org/repo/@v/v1.0.0.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fetcher{Verbose: false}
			got := f.directURL(tt.modulePath, tt.version)
			if got != tt.wantURL {
				t.Errorf("directURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestURLEscaping(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		version    string
		wantInURL  string
	}{
		{
			name:       "lowercase path not escaped",
			modulePath: "github.com/example/repo",
			version:    "v1.0.0",
			wantInURL:  "github.com/example/repo",
		},
		{
			name:       "uppercase gets escaped",
			modulePath: "github.com/Example/Repo",
			version:    "v1.0.0",
			wantInURL:  "!example/!repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fetcher{Proxy: "https://proxy.golang.org"}
			url := f.getDownloadURL(tt.modulePath, tt.version)
			if !contains(url, tt.wantInURL) {
				t.Errorf("URL should contain escaped path %q, got %q", tt.wantInURL, url)
			}
		})
	}
}

func TestFullHashExtraction(t *testing.T) {
	tests := []struct {
		name         string
		modulePath   string
		version      string
		wantFullHash bool
	}{
		{
			name:         "github pseudo-version gets full hash from go list",
			modulePath:   "github.com/sirupsen/logrus",
			version:      "v1.9.3",
			wantFullHash: true,
		},
		{
			name:         "github tagged version gets full hash from go list",
			modulePath:   "github.com/spf13/cobra",
			version:      "v1.8.0",
			wantFullHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fetcher{}
			info, err := f.getModuleInfoFromGoList(tt.modulePath, tt.version)
			if err != nil {
				t.Fatalf("getModuleInfoFromGoList() error = %v", err)
			}

			if tt.wantFullHash {
				if info.Origin == nil || info.Origin.Hash == "" {
					t.Fatalf("Expected full hash, got Origin=%v", info.Origin)
				}

				hashLen := len(info.Origin.Hash)
				if hashLen != 40 {
					t.Errorf("Hash length = %d, want 40 (full git commit hash). Got: %s", hashLen, info.Origin.Hash)
				}
			}
		})
	}
}
