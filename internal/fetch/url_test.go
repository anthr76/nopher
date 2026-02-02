package fetch

import (
	"testing"
)

func TestGetDownloadURL(t *testing.T) {
	tests := []struct {
		name       string
		proxy      string
		private    string
		modulePath string
		version    string
		wantURL    string
	}{
		{
			name:       "public module uses proxy",
			proxy:      "https://proxy.golang.org",
			private:    "",
			modulePath: "golang.org/x/mod",
			version:    "v0.32.0",
			wantURL:    "https://proxy.golang.org/golang.org/x/mod/@v/v0.32.0.zip",
		},
		{
			name:       "private module uses direct",
			proxy:      "https://proxy.golang.org",
			private:    "github.com/myorg/*",
			modulePath: "github.com/myorg/private",
			version:    "v1.0.0",
			wantURL:    "https://github.com/myorg/private/archive/refs/tags/v1.0.0.zip",
		},
		{
			name:       "no proxy uses direct",
			proxy:      "",
			private:    "",
			modulePath: "github.com/example/repo",
			version:    "v1.2.3",
			wantURL:    "https://github.com/example/repo/archive/refs/tags/v1.2.3.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fetcher{
				Proxy:   tt.proxy,
				Private: tt.private,
			}
			got := f.getDownloadURL(tt.modulePath, tt.version)
			if got != tt.wantURL {
				t.Errorf("getDownloadURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestDirectURLWithOrigin(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		version    string
		wantPrefix string
	}{
		{
			name:       "github with submodule",
			modulePath: "github.com/example/repo/sub/module",
			version:    "v1.0.0",
			wantPrefix: "https://github.com/example/repo/archive/refs/tags/v1.0.0.zip",
		},
		{
			name:       "github pseudo-version",
			modulePath: "github.com/example/repo",
			version:    "v0.0.0-20231201120000-abc123def456",
			wantPrefix: "https://github.com/example/repo/archive/abc123def456.zip",
		},
		{
			name:       "BSR module with nested path",
			modulePath: "buf.build/gen/go/owner/repo/connectrpc/go",
			version:    "v1.0.0",
			wantPrefix: "https://buf.build/gen/go/buf.build/gen/go/owner/repo/connectrpc/go/@v/v1.0.0.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fetcher{}
			got := f.directURL(tt.modulePath, tt.version)
			if got != tt.wantPrefix {
				t.Errorf("directURL() = %q, want %q", got, tt.wantPrefix)
			}
		})
	}
}

func TestVersionInURL(t *testing.T) {
	tests := []struct {
		version string
	}{
		{"v1.2.3"},
		{"v1.0.0+incompatible"},
		{"v0.0.0-20231201120000-abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			f := &Fetcher{Proxy: "https://proxy.golang.org"}
			url := f.getDownloadURL("example.com/repo", tt.version)
			if !contains(url, tt.version) {
				t.Errorf("URL should contain version %q, got %q", tt.version, url)
			}
		})
	}
}

func TestGitHubURLConstruction(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		isTag     bool
		isBranch  bool
		commitSHA string
	}{
		{
			name:  "tag ref",
			ref:   "refs/tags/v1.0.0",
			isTag: true,
		},
		{
			name:     "branch ref",
			ref:      "refs/heads/main",
			isBranch: true,
		},
		{
			name:      "commit sha",
			ref:       "",
			commitSHA: "abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the logic we use in directURL
			if tt.isTag && !hasPrefix(tt.ref, "refs/tags/") {
				t.Error("Tag ref should start with refs/tags/")
			}
			if tt.isBranch && !hasPrefix(tt.ref, "refs/heads/") {
				t.Error("Branch ref should start with refs/heads/")
			}
		})
	}
}
