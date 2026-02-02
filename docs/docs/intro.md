---
sidebar_position: 1
slug: /
---

# Introduction

Nopher is a Nix-native Go module builder with first-class private repository support. It provides reproducible Go builds with fine-grained dependency caching.

## Why Nopher?

Traditional approaches to building Go applications in Nix have limitations:

- **Single hash for all dependencies**: Most Nix Go builders use a single fixed-output derivation for all dependencies, meaning any dependency change requires re-downloading everything.
- **Private repository challenges**: Authenticating to private repositories during Nix builds is complex and often requires impure builds.
- **Opacity**: It's difficult to audit which exact versions of dependencies are being used.

Nopher solves these problems by:

1. **Per-module caching**: Each Go module is fetched as a separate Nix derivation, enabling fine-grained caching.
2. **Impure lockfile generation**: Authentication happens during lockfile generation (impure phase), while the actual build is fully pure.
3. **Transparent lockfile**: The YAML lockfile clearly shows every dependency with its version and hash.

## How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                     IMPURE PHASE (with network)                 │
│  ┌─────────┐    ┌──────────┐    ┌─────────────────────────────┐ │
│  │ go.mod  │───▶│  nopher  │───▶│  nopher.lock.yaml           │ │
│  │ go.sum  │    │   CLI    │    │  (Nix SRI hashes per module)│ │
│  └─────────┘    └──────────┘    └─────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     PURE PHASE (no network)                     │
│  ┌─────────────────┐    ┌──────────────────────────────────────┐│
│  │nopher.lock.yaml │───▶│  buildNopherGoApp                    ││
│  └─────────────────┘    │  - fetchGoModule per dependency      ││
│                         │  - Symlink-based vendor assembly     ││
│  ┌─────────────────┐    │  - go build -mod=vendor              ││
│  │  source code    │───▶│                                      ││
│  └─────────────────┘    └──────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Features

- **Fine-grained caching**: Each dependency is cached separately
- **Private repository support**: Works with GitHub, GitLab, Bitbucket, and BSR (Buf Schema Registry)
- **Standard Go tooling**: Uses `go.mod` and `go.sum` as the source of truth
- **Pure Nix builds**: The build phase requires no network access
- **Transparent dependencies**: Human-readable YAML lockfile
- **Replace directive support**: Handles both remote and local replacements
