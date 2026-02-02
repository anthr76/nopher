---
sidebar_position: 1
---

# CLI Reference

Complete reference for the nopher command-line interface.

## Commands

### `nopher generate`

Generate a lockfile from `go.mod` and `go.sum`.

```bash
nopher generate [options] [directory]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-tidy` | Run `go mod tidy` before generating (requires Go in PATH) |
| `-v` | Enable verbose output |

**Examples:**

```bash
# Generate lockfile in current directory
nopher generate

# Generate with verbose output
nopher generate -v

# Run go mod tidy first
nopher generate -tidy

# Generate for a specific directory
nopher generate ./path/to/project
```

### `nopher verify`

Verify that the lockfile matches `go.mod` and `go.sum`.

```bash
nopher verify [directory]
```

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Lockfile is up to date |
| 1 | Lockfile needs regeneration or error occurred |

**Examples:**

```bash
# Verify in current directory
nopher verify

# Verify specific project
nopher verify ./path/to/project

# Use in CI
nopher verify || echo "Lockfile out of date!"
```

### `nopher update`

Update a specific module in the lockfile.

```bash
nopher update <module-path> [directory]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `module-path` | The Go module path to update (e.g., `github.com/sirupsen/logrus`) |
| `directory` | Optional: project directory (default: current directory) |

**Examples:**

```bash
# Update a specific module
nopher update github.com/sirupsen/logrus

# Update module in specific project
nopher update golang.org/x/sys ./path/to/project
```

### `nopher version`

Print version information.

```bash
nopher version
```

### `nopher help`

Show help message.

```bash
nopher help
```

## Environment Variables

Nopher respects standard Go environment variables:

| Variable | Description |
|----------|-------------|
| `GOPROXY` | Go module proxy URL (default: `https://proxy.golang.org`) |
| `GOPRIVATE` | Comma-separated list of private module prefixes |
| `GONOPROXY` | Modules to fetch directly (bypassing proxy) |

**Example:**

```bash
# Use a different proxy
GOPROXY=https://goproxy.io nopher generate

# Mark modules as private
GOPRIVATE=github.com/myorg/* nopher generate
```

## Authentication

For private repositories, nopher reads credentials from `~/.netrc`:

```
machine github.com
  login oauth2
  password ghp_YOUR_TOKEN_HERE

machine gitlab.com
  login oauth2
  password glpat-YOUR_TOKEN_HERE
```

See [Private Repositories](./private-repos) for detailed setup instructions.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error (parse error, network error, etc.) |

## Output Files

### `nopher.lock.yaml`

The primary output file containing all module information:

```yaml
schema: 1
go: "1.22"
modules:
  github.com/example/module:
    version: v1.0.0
    hash: sha256-...=
replace:
  github.com/old/module:
    new: github.com/new/module
    version: v2.0.0
    hash: sha256-...=
```
