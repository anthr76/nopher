---
sidebar_position: 2
---

# Getting Started

This guide will help you set up nopher for your Go project.

## Prerequisites

- [Nix](https://nixos.org/download.html) with flakes enabled
- A Go project with `go.mod` and `go.sum`

## Installation

### Using Nix Flakes

Add nopher to your flake inputs:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    nopher.url = "github:anthr76/nopher";
  };

  outputs = { self, nixpkgs, nopher, ... }:
    let
      system = "x86_64-linux"; # or "aarch64-darwin", etc.
      pkgs = nixpkgs.legacyPackages.${system};
      nopherLib = nopher.lib.${system};
    in {
      packages.${system}.default = nopherLib.buildNopherGoApp {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        modules = ./nopher.lock.yaml;
      };
    };
}
```

### Running the CLI

You can run nopher directly without installation:

```bash
nix run github:anthr76/nopher -- generate
```

Or add it to a dev shell:

```nix
devShells.default = pkgs.mkShell {
  packages = [ nopher.packages.${system}.default ];
};
```

## Quick Start

### 1. Generate the Lockfile

In your Go project directory:

```bash
nopher generate
```

This reads your `go.mod` and `go.sum` files and generates `nopher.lock.yaml`:

```yaml
schema: 1
go: "1.22"
modules:
  github.com/sirupsen/logrus:
    version: v1.9.3
    hash: sha256-E5GnOMrWPCJLof4UFRJ9sLQKLpALbstsrqHmnWpnn5w=
  golang.org/x/sys:
    version: v0.15.0
    hash: sha256-abc123...=
```

### 2. Create Your Flake

Create a `flake.nix` that uses `buildNopherGoApp`:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    nopher.url = "github:anthr76/nopher";
  };

  outputs = { self, nixpkgs, nopher, ... }:
    let
      forAllSystems = nixpkgs.lib.genAttrs [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
    in {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          nopherLib = nopher.lib.${system};
        in {
          default = nopherLib.buildNopherGoApp {
            pname = "myapp";
            version = "1.0.0";
            src = ./.;
            modules = ./nopher.lock.yaml;
            subPackages = [ "./cmd/myapp" ];
          };
        }
      );
    };
}
```

### 3. Build Your Project

```bash
nix build
```

That's it! Your Go application is now built reproducibly with Nix.

## Updating Dependencies

When you update your `go.mod`, regenerate the lockfile:

```bash
go mod tidy
nopher generate
```

Or use the `-tidy` flag to run `go mod tidy` automatically:

```bash
nopher generate -tidy
```

## Next Steps

- [CLI Reference](./usage/cli-reference) - Full CLI documentation
- [Private Repositories](./usage/private-repos) - Set up authentication for private repos
- [Nix Builder Reference](./usage/nix-builder) - All `buildNopherGoApp` options
