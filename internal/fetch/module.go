package fetch

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/git-lfs/go-netrc/netrc"
)

const (
	// DefaultProxy is the default Go module proxy.
	DefaultProxy = "https://proxy.golang.org"
)

// ModuleInfo contains metadata about a module from the .info endpoint
type ModuleInfo struct {
	Version string
	Time    string
	Origin  *struct {
		VCS    string
		URL    string
		Ref    string
		Hash   string
		Subdir string
	}
}

// Fetcher handles fetching Go modules from proxies and direct sources.
type Fetcher struct {
	// Proxy is the GOPROXY URL to use.
	Proxy string
	// Private is a comma-separated list of module path prefixes to fetch directly.
	Private string
	// CacheDir is the directory to cache downloaded modules.
	CacheDir string
	// Netrc contains credentials for private repositories.
	Netrc *netrc.Netrc
	// Verbose enables verbose output.
	Verbose bool
}

// NewFetcher creates a new Fetcher with default settings.
// Reads configuration from environment variables GOPROXY, GOPRIVATE, and GONOPROXY.
// Parses ~/.netrc for authentication credentials.
// Creates cache directory in user's cache dir or temp dir if unavailable.
func NewFetcher() (*Fetcher, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	cacheDir = filepath.Join(cacheDir, "nopher")

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	netrcFile, err := netrc.ParseFile(filepath.Join(home, ".netrc"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("parsing netrc: %w", err)
	}
	if netrcFile == nil {
		netrcFile = &netrc.Netrc{}
	}

	proxy := os.Getenv("GOPROXY")
	if proxy == "" {
		proxy = DefaultProxy
	}
	if idx := strings.Index(proxy, ","); idx != -1 {
		proxy = proxy[:idx]
	}
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
		Netrc:    netrcFile,
	}, nil
}

// FetchResult contains the result of fetching a module.
type FetchResult struct {
	ModulePath string
	Version    string
	Dir        string // Path to extracted module
	Hash       string // SHA256 hash of zip file in SRI format
	URL        string // Source URL used for fetching
	Rev        string // Git commit hash (for GitHub modules)
}

// Fetch downloads a Go module, extracts it, and computes its SRI hash.
// Results are cached in CacheDir keyed by modulePath@version.
// Returns FetchResult with the extracted directory, hash, source URL, and git revision.
func (f *Fetcher) Fetch(modulePath, version string) (*FetchResult, error) {
	cacheKey := escapePath(modulePath) + "@" + version
	cachedDir := filepath.Join(f.CacheDir, cacheKey)
	hashFile := cachedDir + ".hash"
	urlFile := cachedDir + ".url"
	revFile := cachedDir + ".rev"

	if info, err := os.Stat(cachedDir); err == nil && info.IsDir() {
		hashData, hashErr := os.ReadFile(hashFile)
		urlData, urlErr := os.ReadFile(urlFile)
		revData, revErr := os.ReadFile(revFile)
		if hashErr == nil {
			cachedURL := ""
			if urlErr == nil {
				cachedURL = strings.TrimSpace(string(urlData))
			}
			cachedRev := ""
			if revErr == nil {
				cachedRev = strings.TrimSpace(string(revData))
			}
			return &FetchResult{
				ModulePath: modulePath,
				Version:    version,
				Dir:        cachedDir,
				Hash:       strings.TrimSpace(string(hashData)),
				URL:        cachedURL,
				Rev:        cachedRev,
			}, nil
		}
	}

	downloadURL := f.getDownloadURL(modulePath, version)

	zipPath, err := f.downloadFromURL(downloadURL, modulePath, version)
	if err != nil {
		return nil, fmt.Errorf("downloading module: %w", err)
	}
	defer os.Remove(zipPath)

	zipHash, err := computeZipHash(zipPath)
	if err != nil {
		return nil, fmt.Errorf("computing zip hash: %w", err)
	}

	if err := f.extract(zipPath, cachedDir, modulePath, version); err != nil {
		return nil, fmt.Errorf("extracting module: %w", err)
	}

	if err := os.WriteFile(hashFile, []byte(zipHash), 0o644); err != nil && f.Verbose {
		fmt.Fprintf(os.Stderr, "warning: failed to cache hash: %v\n", err)
	}

	if err := os.WriteFile(urlFile, []byte(downloadURL), 0o644); err != nil && f.Verbose {
		fmt.Fprintf(os.Stderr, "warning: failed to cache URL: %v\n", err)
	}

	gitRev := ""
	if strings.HasPrefix(modulePath, "github.com/") {
		var info *ModuleInfo
		var err error

		if f.isPrivate(modulePath) {
			info, err = f.getModuleInfoFromGoList(modulePath, version)
		} else {
			info, _ = f.getModuleInfo(modulePath, version)
			if info == nil {
				info, err = f.getModuleInfoFromGoList(modulePath, version)
			}
		}

		if err == nil && info != nil && info.Origin != nil {
			gitRev = info.Origin.Hash
		}
	}

	if gitRev != "" {
		if err := os.WriteFile(revFile, []byte(gitRev), 0o644); err != nil && f.Verbose {
			fmt.Fprintf(os.Stderr, "warning: failed to cache rev: %v\n", err)
		}
	}

	return &FetchResult{
		ModulePath: modulePath,
		Version:    version,
		Dir:        cachedDir,
		Hash:       zipHash,
		URL:        downloadURL,
		Rev:        gitRev,
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
	if prefix, found := strings.CutSuffix(pattern, "/*"); found {
		return strings.HasPrefix(modulePath, prefix+"/") || modulePath == prefix
	}
	if prefix, found := strings.CutSuffix(pattern, "*"); found {
		return strings.HasPrefix(modulePath, prefix)
	}
	return strings.HasPrefix(modulePath, pattern)
}

// getDownloadURL determines the download URL for a module.
// Private modules use direct URLs, public modules use the configured proxy.
func (f *Fetcher) getDownloadURL(modulePath, version string) string {
	if f.isPrivate(modulePath) {
		return f.directURL(modulePath, version)
	}

	if f.Proxy != "" {
		escapedPath := escapePath(modulePath)
		escapedVersion := escapeVersion(version)
		return fmt.Sprintf("%s/%s/@v/%s.zip", f.Proxy, escapedPath, escapedVersion)
	}

	return f.directURL(modulePath, version)
}

// downloadFromURL fetches a module zip file from the given URL.
// For private modules, adds HTTP Basic Authentication from netrc if available.
// Returns path to temporary zip file that caller must clean up.
func (f *Fetcher) downloadFromURL(downloadURL, modulePath, version string) (string, error) {
	if f.Verbose {
		fmt.Fprintf(os.Stderr, "Downloading %s@%s from %s\n", modulePath, version, downloadURL)
	}

	var client http.Client

	if f.isPrivate(modulePath) {
		host := extractHost(modulePath)
		if machine := f.Netrc.FindMachine(host, ""); machine != nil {
			transport := &authTransport{
				base:     http.DefaultTransport,
				login:    machine.Login,
				password: machine.Password,
			}
			client.Transport = transport
		}
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

// getModuleInfo fetches module metadata from the proxy's .info endpoint.
// Returns nil if proxy is not configured or if the .info endpoint is unavailable.
// Errors are treated as non-fatal and result in nil return.
func (f *Fetcher) getModuleInfo(modulePath, version string) (*ModuleInfo, error) {
	if f.Proxy == "" {
		return nil, nil
	}

	escapedPath := escapePath(modulePath)
	escapedVersion := escapeVersion(version)
	infoURL := fmt.Sprintf("%s/%s/@v/%s.info", f.Proxy, escapedPath, escapedVersion)

	resp, err := http.Get(infoURL)
	if err != nil {
		return nil, nil // Not fatal, just return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil // Not fatal
	}

	var info ModuleInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, nil // Not fatal
	}

	return &info, nil
}

// getModuleInfoFromGoList extracts module metadata from the version string.
// For pseudo-versions (v0.0.0-timestamp-hash), extracts the embedded git commit hash.
// For tagged versions (v1.2.3), constructs the git tag ref (refs/tags/v1.2.3).
// Returns nil for non-GitHub modules.
func (f *Fetcher) getModuleInfoFromGoList(modulePath, version string) (*ModuleInfo, error) {
	// Actually call `go list -m -json` to get accurate Origin data with full commit hash
	cmd := exec.Command("go", "list", "-m", "-json", modulePath+"@"+version)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to manual parsing if go list fails
		return f.getModuleInfoManual(modulePath, version)
	}

	var info ModuleInfo
	if err := json.Unmarshal(output, &info); err != nil {
		// Fallback to manual parsing if JSON parse fails
		return f.getModuleInfoManual(modulePath, version)
	}

	return &info, nil
}

// getModuleInfoManual manually constructs module info from path and version
// This is a fallback when go list is not available
func (f *Fetcher) getModuleInfoManual(modulePath, version string) (*ModuleInfo, error) {
	info := &ModuleInfo{
		Version: version,
	}

	if strings.HasPrefix(modulePath, "github.com/") {
		parts := strings.SplitN(modulePath, "/", 4)
		if len(parts) >= 3 {
			owner := parts[1]
			repoName := parts[2]

			info.Origin = &struct {
				VCS    string
				URL    string
				Ref    string
				Hash   string
				Subdir string
			}{
				VCS: "git",
				URL: fmt.Sprintf("https://github.com/%s/%s", owner, repoName),
			}

			if strings.HasPrefix(version, "v0.0.0-") {
				idx := strings.LastIndex(version, "-")
				if idx != -1 && idx < len(version)-1 {
					info.Origin.Hash = version[idx+1:]
				}
			} else {
				info.Origin.Ref = "refs/tags/" + version
			}
		}
	}

	return info, nil
}

// directURL constructs a direct download URL for a module.
// Routes to the appropriate URL builder based on module type.
func (f *Fetcher) directURL(modulePath, version string) string {
	if strings.HasPrefix(modulePath, "github.com/") {
		return f.buildGitHubURL(modulePath, version)
	}

	if strings.Contains(modulePath, "/gen/go/") {
		return f.buildBSRURL(modulePath, version)
	}

	return f.buildGenericURL(modulePath, version)
}

// buildGitHubURL constructs a GitHub archive download URL.
// Attempts to use Origin metadata for accurate refs/commits, falls back to tag-based URL.
func (f *Fetcher) buildGitHubURL(modulePath, version string) string {
	info := f.getGitHubModuleInfo(modulePath, version)

	if info != nil && info.Origin != nil && info.Origin.VCS == "git" &&
		strings.HasPrefix(info.Origin.URL, "https://github.com/") {

		if archiveURL := f.buildGitHubArchiveURL(info); archiveURL != "" {
			return archiveURL
		}
	}

	parts := strings.SplitN(modulePath, "/", 4)
	if len(parts) >= 3 {
		owner := parts[1]
		repo := parts[2]
		return fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/%s.zip", owner, repo, version)
	}

	return f.buildGenericURL(modulePath, version)
}

// getGitHubModuleInfo retrieves module metadata for GitHub repositories.
// For private repos, uses getModuleInfoFromGoList (authenticated).
// For public repos, tries proxy .info endpoint first, then falls back to getModuleInfoFromGoList.
func (f *Fetcher) getGitHubModuleInfo(modulePath, version string) *ModuleInfo {
	if f.isPrivate(modulePath) {
		info, _ := f.getModuleInfoFromGoList(modulePath, version)
		return info
	}

	if info, _ := f.getModuleInfo(modulePath, version); info != nil && info.Origin != nil {
		return info
	}

	info, _ := f.getModuleInfoFromGoList(modulePath, version)
	return info
}

// buildGitHubArchiveURL constructs a GitHub archive download URL from Origin metadata.
// Handles tag refs (refs/tags/*), branch refs (refs/heads/*), and commit hashes.
// Returns empty string if no valid ref or hash is found.
func (f *Fetcher) buildGitHubArchiveURL(info *ModuleInfo) string {
	repoURL := strings.TrimPrefix(info.Origin.URL, "https://github.com/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	if tag, found := strings.CutPrefix(info.Origin.Ref, "refs/tags/"); found {
		if f.Verbose {
			fmt.Fprintf(os.Stderr, "Using tag %s from module info\n", tag)
		}
		return fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.zip", repoURL, url.PathEscape(tag))
	}

	if branch, found := strings.CutPrefix(info.Origin.Ref, "refs/heads/"); found {
		if f.Verbose {
			fmt.Fprintf(os.Stderr, "Using branch %s from module info\n", branch)
		}
		return fmt.Sprintf("https://github.com/%s/archive/refs/heads/%s.zip", repoURL, url.PathEscape(branch))
	}

	if info.Origin.Hash != "" {
		if f.Verbose {
			fmt.Fprintf(os.Stderr, "Using commit hash %s from module info\n", info.Origin.Hash)
		}
		return fmt.Sprintf("https://github.com/%s/archive/%s.zip", repoURL, info.Origin.Hash)
	}

	return ""
}

// buildBSRURL constructs a Buf Schema Registry (BSR) download URL.
// BSR modules have paths like buf.build/gen/go/org/repo and use the full path in the URL.
func (f *Fetcher) buildBSRURL(modulePath, version string) string {
	host := extractHost(modulePath)
	escapedPath := escapePath(modulePath)
	escapedVersion := escapeVersion(version)
	return fmt.Sprintf("https://%s/gen/go/%s/@v/%s.zip", host, escapedPath, escapedVersion)
}

// buildGenericURL constructs a generic module proxy URL.
// Used for self-hosted registries and other non-GitHub, non-BSR modules.
func (f *Fetcher) buildGenericURL(modulePath, version string) string {
	host := extractHost(modulePath)
	escapedPath := escapePath(modulePath)
	escapedVersion := escapeVersion(version)
	return fmt.Sprintf("https://%s/%s/@v/%s.zip", host, escapedPath, escapedVersion)
}

// extract unpacks a module zip to the target directory.
// Module zips contain files under modulePath@version/ prefix which is stripped during extraction.
// Handles archives with non-standard directory structures by stripping the first path segment.
func (f *Fetcher) extract(zipPath, targetDir, modulePath, version string) error {
	os.RemoveAll(targetDir)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	prefix := modulePath + "@" + version + "/"

	for _, file := range r.File {
		name := file.Name
		if after, found := strings.CutPrefix(name, prefix); found {
			name = after
		} else {
			if _, after, found := strings.Cut(name, "/"); found {
				name = after
			}
		}

		if name == "" {
			continue
		}

		targetPath := filepath.Join(targetDir, name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("creating parent directory: %w", err)
		}

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
	host, _, found := strings.Cut(modulePath, "/")
	if found {
		return host
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
