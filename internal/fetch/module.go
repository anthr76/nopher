package fetch

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultProxy is the default Go module proxy.
	DefaultProxy = "https://proxy.golang.org"
)

// Fetcher handles fetching Go modules from proxies and direct sources.
type Fetcher struct {
	// Proxy is the GOPROXY URL to use.
	Proxy string
	// Private is a comma-separated list of module path prefixes to fetch directly.
	Private string
	// CacheDir is the directory to cache downloaded modules.
	CacheDir string
	// Netrc contains credentials for private repositories.
	Netrc *Netrc
	// Verbose enables verbose output.
	Verbose bool
}

// NewFetcher creates a new Fetcher with default settings.
func NewFetcher() (*Fetcher, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	cacheDir = filepath.Join(cacheDir, "nopher")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	netrc, err := ParseNetrc()
	if err != nil {
		return nil, fmt.Errorf("parsing netrc: %w", err)
	}

	proxy := os.Getenv("GOPROXY")
	if proxy == "" {
		proxy = DefaultProxy
	}
	// Handle comma-separated proxy list - use first one
	if idx := strings.Index(proxy, ","); idx != -1 {
		proxy = proxy[:idx]
	}
	// Handle "direct" and "off"
	if proxy == "direct" || proxy == "off" {
		proxy = ""
	}

	private := os.Getenv("GOPRIVATE")
	if private == "" {
		private = os.Getenv("GONOPROXY")
	}

	return &Fetcher{
		Proxy:    proxy,
		Private:  private,
		CacheDir: cacheDir,
		Netrc:    netrc,
	}, nil
}

// FetchResult contains the result of fetching a module.
type FetchResult struct {
	ModulePath string
	Version    string
	Dir        string // Path to extracted module
	Hash       string // SHA256 hash of zip file in SRI format
}

// Fetch downloads a module and computes its hash.
func (f *Fetcher) Fetch(modulePath, version string) (*FetchResult, error) {
	// Check cache first
	cacheKey := escapePath(modulePath) + "@" + version
	cachedDir := filepath.Join(f.CacheDir, cacheKey)
	hashFile := cachedDir + ".hash"

	if info, err := os.Stat(cachedDir); err == nil && info.IsDir() {
		if hashData, err := os.ReadFile(hashFile); err == nil {
			return &FetchResult{
				ModulePath: modulePath,
				Version:    version,
				Dir:        cachedDir,
				Hash:       strings.TrimSpace(string(hashData)),
			}, nil
		}
	}

	// Download the module
	zipPath, err := f.download(modulePath, version)
	if err != nil {
		return nil, fmt.Errorf("downloading module: %w", err)
	}
	defer os.Remove(zipPath)

	// Compute SHA256 hash of the zip file (what Nix fetchurl expects)
	zipHash, err := computeZipHash(zipPath)
	if err != nil {
		return nil, fmt.Errorf("computing zip hash: %w", err)
	}

	// Extract the module
	if err := f.extract(zipPath, cachedDir, modulePath, version); err != nil {
		return nil, fmt.Errorf("extracting module: %w", err)
	}

	// Cache the hash
	if err := os.WriteFile(hashFile, []byte(zipHash), 0644); err != nil {
		// Non-fatal, just log if verbose
		if f.Verbose {
			fmt.Fprintf(os.Stderr, "warning: failed to cache hash: %v\n", err)
		}
	}

	return &FetchResult{
		ModulePath: modulePath,
		Version:    version,
		Dir:        cachedDir,
		Hash:       zipHash,
	}, nil
}

// computeZipHash computes the SHA256 hash of a file in SRI format.
func computeZipHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// isPrivate checks if a module path should be fetched directly (not via proxy).
func (f *Fetcher) isPrivate(modulePath string) bool {
	if f.Private == "" {
		return false
	}

	for _, pattern := range strings.Split(f.Private, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if matchPattern(pattern, modulePath) {
			return true
		}
	}
	return false
}

// matchPattern checks if modulePath matches a GOPRIVATE pattern.
func matchPattern(pattern, modulePath string) bool {
	// Handle wildcard patterns
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(modulePath, prefix+"/") || modulePath == prefix
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(modulePath, prefix)
	}
	return strings.HasPrefix(modulePath, pattern)
}

// download fetches the module zip file.
func (f *Fetcher) download(modulePath, version string) (string, error) {
	var downloadURL string
	var client http.Client

	if f.isPrivate(modulePath) {
		// Direct download from source
		downloadURL = f.directURL(modulePath, version)

		// Set up authentication if available
		host := extractHost(modulePath)
		if entry := f.Netrc.FindEntry(host); entry != nil {
			transport := &authTransport{
				base:     http.DefaultTransport,
				login:    entry.Login,
				password: entry.Password,
			}
			client.Transport = transport
		}
	} else if f.Proxy != "" {
		// Use proxy
		escapedPath := escapePath(modulePath)
		escapedVersion := escapeVersion(version)
		downloadURL = fmt.Sprintf("%s/%s/@v/%s.zip", f.Proxy, escapedPath, escapedVersion)
	} else {
		// No proxy configured, try direct
		downloadURL = f.directURL(modulePath, version)
	}

	if f.Verbose {
		fmt.Fprintf(os.Stderr, "Downloading %s@%s from %s\n", modulePath, version, downloadURL)
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching module: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "nopher-*.zip")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("downloading: %w", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

// directURL constructs a direct download URL for a module.
func (f *Fetcher) directURL(modulePath, version string) string {
	// For GitHub, construct the archive URL
	if strings.HasPrefix(modulePath, "github.com/") {
		parts := strings.SplitN(modulePath, "/", 4)
		if len(parts) >= 3 {
			owner := parts[1]
			repo := parts[2]
			// Version might be v1.2.3 or v0.0.0-timestamp-hash
			return fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/%s.zip", owner, repo, version)
		}
	}

	// For any module (including BSR, self-hosted registries, etc.)
	// Use the standard Go module proxy URL format
	host := extractHost(modulePath)
	escapedPath := escapePath(modulePath)
	escapedVersion := escapeVersion(version)
	return fmt.Sprintf("https://%s/%s/@v/%s.zip", host, strings.TrimPrefix(escapedPath, escapePath(host+"/")), escapedVersion)
}

// extract unpacks a module zip to the target directory.
func (f *Fetcher) extract(zipPath, targetDir, modulePath, version string) error {
	// Remove existing directory if present
	os.RemoveAll(targetDir)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	// Module zips contain files under modulePath@version/ prefix
	prefix := modulePath + "@" + version + "/"

	for _, file := range r.File {
		// Strip the module@version prefix
		name := file.Name
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
		} else {
			// Some zips might have different structure
			// Try to find any prefix ending with /
			if idx := strings.Index(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
		}

		if name == "" {
			continue
		}

		targetPath := filepath.Join(targetDir, name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("creating parent directory: %w", err)
		}

		// Extract file
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("opening zip entry: %w", err)
		}

		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return fmt.Errorf("creating file: %w", err)
		}

		_, err = io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			return fmt.Errorf("extracting file: %w", err)
		}
	}

	return nil
}

// escapePath escapes a module path for use in URLs.
func escapePath(path string) string {
	// Go module proxy encodes uppercase letters
	var result strings.Builder
	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			result.WriteRune('!')
			result.WriteRune(r + ('a' - 'A'))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// escapeVersion escapes a version for use in URLs.
func escapeVersion(version string) string {
	return url.PathEscape(version)
}

// extractHost gets the host part of a module path.
func extractHost(modulePath string) string {
	if idx := strings.Index(modulePath, "/"); idx != -1 {
		return modulePath[:idx]
	}
	return modulePath
}

// authTransport adds basic auth to HTTP requests.
type authTransport struct {
	base     http.RoundTripper
	login    string
	password string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.login, t.password)
	return t.base.RoundTrip(req)
}
