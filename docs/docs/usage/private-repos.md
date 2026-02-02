---
sidebar_position: 3
---

# Private Repositories

Nopher supports private Go modules from various sources including GitHub, GitLab, Bitbucket, and Buf Schema Registry (BSR).

## How It Works

Private repository authentication is handled during the **impure phase** (lockfile generation). The actual Nix build is fully **pure** and requires no authentication - all module hashes are pre-computed in the lockfile.

```
┌──────────────────────────────────────────────────────────────────┐
│                     IMPURE PHASE (authenticated)                 │
│                                                                  │
│  nopher generate                                                 │
│    │                                                             │
│    ├─► Public modules: fetch via proxy.golang.org                │
│    │                                                             │
│    └─► Private modules (GOPRIVATE):                              │
│          • Read ~/.netrc for credentials                         │
│          • Fetch directly from source                            │
│          • Compute hash for lockfile                             │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│                     PURE PHASE (no auth needed)                  │
│                                                                  │
│  nix build                                                       │
│    │                                                             │
│    └─► All modules fetched via proxy.golang.org                  │
│        (private modules must be available via proxy or           │
│         the lockfile provides the direct download hash)          │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

## Setting Up Authentication

### 1. Configure GOPRIVATE

Tell Go which modules are private:

```bash
# In your shell profile (.bashrc, .zshrc, etc.)
export GOPRIVATE="github.com/myorg/*,gitlab.mycompany.com/*"
```

Common patterns:

- `github.com/myorg/*` - All repos in an organization
- `github.com/myorg/myrepo` - Specific repository
- `*.internal.company.com` - All subdomains

### 2. Create ~/.netrc

Add credentials for each private host:

```
# GitHub (using personal access token)
machine github.com
  login oauth2
  password ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# GitLab (using personal access token)
machine gitlab.com
  login oauth2
  password glpat-xxxxxxxxxxxxxxxxxxxx

# Self-hosted GitLab
machine gitlab.mycompany.com
  login oauth2
  password glpat-xxxxxxxxxxxxxxxxxxxx

# Bitbucket (using app password)
machine bitbucket.org
  login your-username
  password your-app-password

# Buf Schema Registry
machine buf.build
  login your-username
  password your-bsr-token
```

**Important:** Protect your netrc file:

```bash
chmod 600 ~/.netrc
```

### 3. Generate the Lockfile

```bash
nopher generate -v
```

The verbose flag shows which modules are being fetched and from where.

## Provider-Specific Setup

### GitHub

1. Create a Personal Access Token (PAT):
   - Go to GitHub → Settings → Developer settings → Personal access tokens
   - Create a token with `repo` scope (for private repos)

2. Add to `~/.netrc`:

   ```
   machine github.com
     login oauth2
     password ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
   ```

### GitLab

1. Create a Personal Access Token:
   - Go to GitLab → Preferences → Access Tokens
   - Create a token with `read_api` and `read_repository` scopes

2. Add to `~/.netrc`:

   ```
   machine gitlab.com
     login oauth2
     password glpat-xxxxxxxxxxxxxxxxxxxx
   ```

### Buf Schema Registry (BSR)

BSR hosts generated Go code from Protocol Buffer definitions.

1. Get your BSR token from [buf.build](https://buf.build)

2. Add to `~/.netrc`:

   ```shell
   machine buf.build
     login your-username
     password your-bsr-token
   ```

3. Add BSR modules to GOPRIVATE:

   ```bash
   export GOPRIVATE="buf.build/gen/go/*"
   ```

### Self-Hosted Registries

For self-hosted Git servers or module proxies:

1. Add the host to GOPRIVATE:

   ```bash
   export GOPRIVATE="git.mycompany.com/*"
   ```

2. Add credentials to `~/.netrc`:

   ```shell
   machine git.mycompany.com
     login your-username
     password your-password-or-token
   ```

## CI/CD Integration

### GitHub Actions

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Configure Git credentials
        run: |
          echo "machine github.com login oauth2 password ${{ secrets.GH_PAT }}" > ~/.netrc
          chmod 600 ~/.netrc

      - name: Install Nix
        uses: cachix/install-nix-action@v24

      - name: Generate lockfile
        env:
          GOPRIVATE: github.com/myorg/*
        run: nix run github:anthr76/nopher -- generate

      - name: Build
        run: nix build
```

### GitLab CI

```yaml
build:
  image: nixos/nix
  before_script:
    - echo "machine gitlab.com login oauth2 password ${CI_JOB_TOKEN}" > ~/.netrc
    - chmod 600 ~/.netrc
    - export GOPRIVATE="gitlab.com/myorg/*"
  script:
    - nix run github:anthr76/nopher -- generate
    - nix build
```

## Troubleshooting

### "410 Gone" or "404 Not Found"

The module is marked as private but credentials are missing or invalid.

1. Verify GOPRIVATE includes the module path:

   ```bash
   echo $GOPRIVATE
   ```

2. Test authentication manually:

   ```bash
   curl -n https://github.com/myorg/private-repo
   ```

### "x509: certificate signed by unknown authority"

For self-hosted servers with custom certificates:

```bash
export SSL_CERT_FILE=/path/to/ca-bundle.crt
nopher generate
```

### Module Not Appearing in Lockfile

1. Run with verbose output:

   ```bash
   nopher generate -v
   ```

2. Ensure the module is in `go.sum`:

   ```bash
   go mod tidy
   nopher generate
   ```

### Cache Issues

Clear the nopher cache:

```bash
rm -rf ~/.cache/nopher
nopher generate
```
