---
sidebar_position: 1
---

# Lockfile Format

The `nopher.lock.yaml` file contains all the information needed to reproducibly fetch Go dependencies.

## Schema

```yaml
schema: 1
go: "1.22"
modules:
  <module-path>:
    version: <version>
    hash: <sri-hash>
replace:
  <original-path>:
    new: <replacement-path>
    version: <version>
    hash: <sri-hash>
  <original-path>:
    path: <local-path>
```

## Fields

### `schema`

**Type:** integer
**Required:** yes

The lockfile schema version. Currently `1`.

```yaml
schema: 1
```

### `go`

**Type:** string
**Required:** yes

The Go version from `go.mod`. Used to set the language level for vendored modules.

```yaml
go: "1.22"
```

### `modules`

**Type:** map
**Required:** no (but typically present)

Map of module paths to their version and hash information.

```yaml
modules:
  github.com/sirupsen/logrus:
    version: v1.9.3
    hash: sha256-E5GnOMrWPCJLof4UFRJ9sLQKLpALbstsrqHmnWpnn5w=
    url: https://github.com/sirupsen/logrus/archive/refs/tags/v1.9.3.zip
    rev: 3d4380f53a34dcdc95f0c1db702615992b38d9a4
  golang.org/x/sys:
    version: v0.15.0
    hash: sha256-abc123def456...=
```

#### Module Entry Fields

| Field     | Type   | Required | Description                                         |
|-----------|--------|----------|-----------------------------------------------------|
| `version` | string | Yes      | Semantic version (e.g., `v1.2.3`) or pseudo-version |
| `hash`    | string | Yes      | SRI hash of the module zip file                     |
| `url`     | string | No       | Direct download URL (used for GitHub fetchGit)      |
| `rev`     | string | No       | Git commit hash for reproducible fetchGit builds    |

**Note:** The `url` and `rev` fields are automatically populated for GitHub modules and used by Nix's `fetchGit` to enable netrc authentication for private repositories.

### `replace`

**Type:** map
**Required:** no

Map of replaced module paths. Corresponds to `replace` directives in `go.mod`.

#### Remote Replacement

When one module is replaced with another remote module:

```yaml
replace:
  sigs.k8s.io/controller-runtime:
    old: sigs.k8s.io/controller-runtime
    oldVersion: v0.23.0
    new: sigs.k8s.io/controller-runtime
    version: v0.22.4
    hash: sha256-xyz789...=
    url: https://proxy.golang.org/sigs.k8s.io/controller-runtime/@v/v0.22.4.zip
```

#### Remote Replacement Fields

| Field        | Type   | Required | Description                                    |
|--------------|--------|----------|------------------------------------------------|
| `old`        | string | No       | Original module path (usually same as key)     |
| `oldVersion` | string | No       | Original version being replaced from go.mod    |
| `new`        | string | Yes      | Replacement module path                        |
| `version`    | string | Yes      | Replacement module version                     |
| `hash`       | string | Yes      | SRI hash of the replacement module zip         |
| `url`        | string | No       | Direct download URL (for GitHub modules)       |
| `rev`        | string | No       | Git commit hash (for GitHub fetchGit)          |

**Note:** The `old` and `oldVersion` fields are used to generate correct `vendor/modules.txt` format that Go expects.

#### Local Replacement

When a module is replaced with a local path:

```yaml
replace:
  github.com/myorg/shared:
    path: ./internal/shared
```

Local replacements don't have a hash because they're part of the source tree.

## Hash Format

Hashes use the [Subresource Integrity (SRI)](https://developer.mozilla.org/en-US/docs/Web/Security/Subresource_Integrity) format:

```shell
sha256-<base64-encoded-hash>
```

The hash is computed over the module's `.zip` file as downloaded from the Go module proxy.

**Example:**

```yaml
hash: sha256-E5GnOMrWPCJLof4UFRJ9sLQKLpALbstsrqHmnWpnn5w=
```

## Complete Example

```yaml
schema: 1
go: "1.22"

modules:
  github.com/sirupsen/logrus:
    version: v1.9.3
    hash: sha256-E5GnOMrWPCJLof4UFRJ9sLQKLpALbstsrqHmnWpnn5w=

  golang.org/x/sys:
    version: v0.15.0
    hash: sha256-KV/KG7OkT7RgnfJLFI/hXfebnjbdqgfSkJLp3XL2PQI=

  google.golang.org/protobuf:
    version: v1.31.0
    hash: sha256-5BPrCp0Wzhz8NLqg2jP7rnQnXBJL5qN6PVPx7VSP7ms=

  # Private module (fetched during lockfile generation)
  github.com/myorg/internal-lib:
    version: v0.5.2
    hash: sha256-abc123def456ghi789...=
    url: https://github.com/myorg/internal-lib/archive/refs/tags/v0.5.2.zip
    rev: def456abc123...

replace:
  # Fork replacement
  github.com/original/pkg:
    new: github.com/myorg/pkg-fork
    version: v1.0.0-fork
    hash: sha256-xyz789...=

  # Local development override
  github.com/myorg/shared:
    path: ./libs/shared
```

## Version Formats

### Semantic Versions

Standard semantic versions:

```yaml
version: v1.2.3
version: v0.0.1
version: v2.0.0
```

### Pre-release Versions

```yaml
version: v1.0.0-alpha.1
version: v2.0.0-beta
version: v1.0.0-rc.1
```

### Pseudo-versions

For commits without tags:

```yaml
version: v0.0.0-20231201120000-abcdef123456
```

Format: `vX.Y.Z-YYYYMMDDHHMMSS-<commit-hash>`

### Incompatible Versions

For major versions ≥ 2 without go.mod:

```yaml
version: v3.0.0+incompatible
```

## Lockfile Generation

The lockfile is generated by:

1. Parsing `go.mod` for direct dependencies and replacements
2. Parsing `go.sum` for the complete dependency graph
3. Fetching each module (via proxy or direct for private modules)
4. Computing the SRI hash of each module's zip file
5. Writing the YAML lockfile

## Lockfile Verification

Use `nopher verify` to check if the lockfile matches `go.mod`/`go.sum`:

```bash
nopher verify
```

This compares:

- Module paths and versions in the lockfile vs `go.sum`
- Replace directives in the lockfile vs `go.mod`

## Best Practices

1. **Commit the lockfile**: `nopher.lock.yaml` should be committed to version control
2. **Regenerate after updates**: Run `nopher generate` after `go get` or `go mod tidy`
3. **CI verification**: Run `nopher verify` in CI to catch lockfile drift
4. **Review changes**: Lockfile diffs show exactly which dependencies changed
