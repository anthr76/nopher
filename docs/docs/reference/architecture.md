---
sidebar_position: 2
---

# Architecture

This document describes the internal architecture of nopher.

## Overview

Nopher consists of two main components:

1. **CLI Tool** (Go): Generates lockfiles from Go modules
2. **Nix Builder**: Consumes lockfiles to build Go applications

## CLI Architecture

```shell
cmd/nopher/
├── main.go          # CLI entry point
└── cmd/
    ├── root.go      # Root command setup
    ├── generate.go  # Generate lockfile command
    ├── verify.go    # Verify lockfile command
    └── update.go    # Update single module command

internal/
├── mod/
│   └── parser.go    # go.mod and go.sum parsing
├── fetch/
│   └── module.go    # Module fetching with multi-source support
│                    # (proxy.golang.org, GitHub, BSR, go list)
├── hash/
│   ├── convert.go   # Hash computation and conversion
│   └── nar.go       # NAR hash support
└── lockfile/
    ├── schema.go    # Lockfile type definitions
    └── yaml.go      # YAML marshaling/unmarshaling
```

### Command Flow

```shell
┌──────────────────────────────────────────────────────────────┐
│                    nopher generate                           │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Parse go.mod                                             │
│     └─► Extract module path, Go version, requires, replaces  │
│                                                              │
│  2. Parse go.sum                                             │
│     └─► Extract all module versions with h1: hashes          │
│                                                              │
│  3. For each module:                                         │
│     ├─► Check if private (GOPRIVATE)                         │
│     ├─► Fetch via proxy or direct                            │
│     ├─► Compute SHA256 hash of zip                           │
│     └─► Add to lockfile                                      │
│                                                              │
│  4. Handle replacements                                      │
│     ├─► Remote: fetch and hash replacement module            │
│     └─► Local: record path (no hash needed)                  │
│                                                              │
│  5. Write nopher.lock.yaml                                   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### Module Fetching

```go
type Fetcher struct {
    Proxy    string   // GOPROXY URL
    Private  string   // GOPRIVATE patterns
    CacheDir string   // Local cache directory
    Netrc    *Netrc   // Authentication credentials
    Verbose  bool     // Enable verbose output
}
```

The fetcher:

1. Checks if module matches GOPRIVATE patterns
2. For public modules: fetches from GOPROXY (proxy.golang.org)
3. For private GitHub modules:
   - Calls `go list -m -json` to get full commit hash and accurate tag/ref
   - Fetches from GitHub archive URLs with netrc authentication
   - Stores both URL and full 40-char commit hash in lockfile
4. For BSR modules: fetches with full module path in URL
5. Caches downloaded modules, URLs, and git revs locally

### Hash Computation

Nopher computes the SHA256 hash of the module zip file:

```go
func computeZipHash(path string) (string, error) {
    f, err := os.Open(path)
    // ...
    h := sha256.New()
    io.Copy(h, f)
    return "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
```

This hash format (SRI) is compatible with Nix's `fetchurl`.

## Nix Builder Architecture

```shell
nix/
├── default.nix              # Main entry, exports
├── fetch-module.nix         # fetchGoModule function
├── build-nopher-go-app.nix  # buildNopherGoApp function
├── lib.nix                  # Helper functions
└── overlay.nix              # nixpkgs overlay
```

### Build Flow

```shell
┌──────────────────────────────────────────────────────────────┐
│                    buildNopherGoApp                          │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Parse lockfile (YAML → JSON via IFD)                     │
│     └─► Uses yj tool at eval time                            │
│                                                              │
│  2. Fetch modules (fetchGoModule for each)                   │
│     └─► Each module is a separate fixed-output derivation    │
│                                                              │
│  3. Assemble vendor directory                                │
│     ├─► Create directory structure                           │
│     ├─► Symlink fetched modules                              │
│     └─► Generate vendor/modules.txt                          │
│                                                              │
│  4. Build application                                        │
│     ├─► Copy vendor (dereference symlinks)                   │
│     ├─► Remove go.mod files from vendor                      │
│     ├─► go build -mod=vendor                                 │
│     └─► Install binaries                                     │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### fetchGoModule

Fetches a single Go module as a fixed-output derivation:

```nix
fetchGoModule {
  modulePath = "github.com/sirupsen/logrus";
  version = "v1.9.3";
  hash = "sha256-E5GnOMrWPCJLof4UFRJ9sLQKLpALbstsrqHmnWpnn5w=";
  url = "https://github.com/sirupsen/logrus/archive/refs/tags/v1.9.3.zip";  # Optional
  rev = "3d4380f53a34dcdc95f0c1db702615992b38d9a4";  # Optional
}
```

**Fetching Strategy:**

The `fetchGoModule` function intelligently selects the fetching method based on module type:

- **GitHub modules with full `rev`**: Uses `builtins.fetchGit`
  - Authenticates via netrc configured in `/etc/nix/nix.conf` (netrc-file setting)
  - Works in pure evaluation mode with full 40-character commit hash
  - Supports multi-module repositories (extracts subdirectories)
  - Example: Private GitHub repos, forks, submodules

- **GitHub modules without full `rev`**: Falls back to `fetchurlBoot`
  - Used when rev is missing or truncated
  - Downloads from proxy.golang.org or GitHub archive URL

- **BSR modules**: Uses `builtins.fetchurl`
  - Authenticates via netrc-file setting in nix.conf
  - Constructs URLs with full module path
  - Example: `https://bsr.host.com/gen/go/bsr.host.com/gen/go/org/repo/@v/version.zip`

- **Other modules**: Uses `fetchurlBoot`
  - Downloads from proxy.golang.org
  - Standard public Go modules

### Vendor Directory Assembly

The vendor directory is built via symlinks for efficiency:

```nix
vendorDir = stdenv.mkDerivation {
  name = "${pname}-vendor";
  installPhase = ''
    mkdir -p $out
    # Symlink each fetched module
    # For modules with children (e.g., github.com/aws/aws-sdk-go-v2 and
    # github.com/aws/aws-sdk-go-v2/config), copy the parent instead of
    # symlinking to avoid permission issues
    ${lib.concatStringsSep "\n" (lib.mapAttrsToList (path: drv: ''
      mkdir -p $out/${dirOf path}
      ln -s ${drv} $out/${path}
    '') fetchedModules)}

    # Generate modules.txt with replace directives
    # Format: # original@version => replacement@version
    # ...
  '';
};
```

**Nested Module Handling:**
- Detects parent-child module relationships (e.g., `aws/sdk` and `aws/sdk/config`)
- Parent modules are copied (not symlinked) to allow child symlinks
- Ensures permissions are correct for nested structures

### modules.txt Generation

Go's vendor mode requires `vendor/modules.txt`:

```shell
# github.com/sirupsen/logrus v1.9.3
## explicit; go 1.22
github.com/sirupsen/logrus
github.com/sirupsen/logrus/hooks/syslog

# sigs.k8s.io/controller-runtime v0.23.0 => sigs.k8s.io/controller-runtime v0.22.4
## explicit; go 1.22
sigs.k8s.io/controller-runtime
sigs.k8s.io/controller-runtime/pkg/client
```

Key points:

- `## explicit` marks the module as explicitly required
- `; go X.Y` sets the language version (prevents go1.16 default)
- Replace directives show `original@version => replacement@version`
- Original version comes from `oldVersion` field in lockfile
- Package paths are discovered by scanning for `.go` files in each module

## Design Decisions

### Per-Module Fetching

**Why:** Fine-grained caching. When one dependency updates, only that module is re-fetched.

**Trade-off:** More derivations, potentially slower initial builds.

### YAML Lockfile

**Why:** Human-readable, easy to review in PRs, can be manually edited if needed.

**Trade-off:** Requires YAML→JSON conversion in Nix (uses IFD with `yj`).

### Impure Lockfile Generation

**Why:** Authentication for private repos is handled outside Nix, keeping builds pure.

**Trade-off:** Lockfile must be regenerated manually after dependency changes.

### Vendor Mode

**Why:** Offline builds, no network during Nix build phase.

**Trade-off:** Larger vendor directories, go.mod file removal needed.

### SRI Hashes

**Why:** Compatible with Nix's fetchurl, standard format.

**Trade-off:** Different from Go's h1: hashes, requires download to compute.

## Caching Strategy

```shell
~/.cache/nopher/  (or ~/Library/Caches/nopher on macOS)
├── github.com%2Fsirupsen%2Flogrus@v1.9.3/    # Extracted module
├── github.com%2Fsirupsen%2Flogrus@v1.9.3.hash # Cached SRI hash
├── github.com%2Fsirupsen%2Flogrus@v1.9.3.url  # Cached source URL
├── github.com%2Fsirupsen%2Flogrus@v1.9.3.rev  # Cached git commit hash
└── ...
```

- Modules are cached after first fetch
- Hash, URL, and git rev are cached alongside module
- Speeds up lockfile regeneration for unchanged dependencies
- Cache location: `~/.cache/nopher` (Linux) or `~/Library/Caches/nopher` (macOS)
- Cache can be cleared: `rm -rf ~/.cache/nopher` or `rm -rf ~/Library/Caches/nopher`

## Error Handling

The CLI provides detailed error messages:

```shell
Error: fetching module github.com/private/repo@v1.0.0:
  unexpected status: 404 Not Found

Hint: This module may be private. Ensure:
  1. GOPRIVATE includes github.com/private/*
  2. ~/.netrc contains credentials for github.com
```
