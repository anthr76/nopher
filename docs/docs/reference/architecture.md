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
└── main.go          # CLI entry point, command routing

internal/
├── mod/
│   └── parser.go    # go.mod and go.sum parsing
├── fetch/
│   ├── module.go    # Module fetching via GOPROXY
│   └── netrc.go     # .netrc credential parsing
├── hash/
│   └── convert.go   # Hash computation and conversion
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
}
```

The fetcher:

1. Checks if module matches GOPRIVATE patterns
2. For public modules: fetches from GOPROXY
3. For private modules: fetches directly with netrc authentication
4. Caches downloaded modules locally

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
}
```

Internally uses `fetchurl` to download from `proxy.golang.org`.

### Vendor Directory Assembly

The vendor directory is built via symlinks for efficiency:

```nix
vendorDir = stdenv.mkDerivation {
  name = "${pname}-vendor";
  installPhase = ''
    mkdir -p $out
    # Symlink each fetched module
    ${lib.concatStringsSep "\n" (lib.mapAttrsToList (path: drv: ''
      mkdir -p $out/${dirOf path}
      ln -s ${drv} $out/${path}
    '') fetchedModules)}

    # Generate modules.txt
    # ...
  '';
};
```

### modules.txt Generation

Go's vendor mode requires `vendor/modules.txt`:

```shell
# github.com/sirupsen/logrus v1.9.3
## explicit; go 1.22
github.com/sirupsen/logrus
github.com/sirupsen/logrus/hooks/syslog
```

Key points:

- `## explicit` marks the module as explicitly required
- `; go X.Y` sets the language version (prevents go1.16 default)
- Package paths are discovered by scanning for `.go` files

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
~/.cache/nopher/
├── github.com%2Fsirupsen%2Flogrus@v1.9.3/    # Extracted module
├── github.com%2Fsirupsen%2Flogrus@v1.9.3.hash # Cached hash
└── ...
```

- Modules are cached after first fetch
- Hash is cached alongside module
- Cache can be cleared: `rm -rf ~/.cache/nopher`

## Error Handling

The CLI provides detailed error messages:

```shell
Error: fetching module github.com/private/repo@v1.0.0:
  unexpected status: 404 Not Found

Hint: This module may be private. Ensure:
  1. GOPRIVATE includes github.com/private/*
  2. ~/.netrc contains credentials for github.com
```
