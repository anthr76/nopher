---
sidebar_position: 2
---

# Nix Builder Reference

Complete reference for the `buildNopherGoApp` Nix function.

## Basic Usage

```nix
buildNopherGoApp {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
}
```

## Parameters

### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `pname` | string | Package name |
| `version` | string | Package version |
| `src` | path | Source directory |
| `modules` | path | Path to `nopher.lock.yaml` |

### Build Options

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `subPackages` | list of strings | `["."]` | Go packages to build |
| `ldflags` | list of strings | `[]` | Linker flags |
| `tags` | list of strings | `[]` | Build tags |

### Go Environment

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `CGO_ENABLED` | string | `"0"` | Enable/disable CGO |
| `GOOS` | string | `null` | Target operating system |
| `GOARCH` | string | `null` | Target architecture |

### Hooks

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `preBuild` | string | `""` | Script to run before build |
| `postBuild` | string | `""` | Script to run after build |
| `preInstall` | string | `""` | Script to run before install |
| `postInstall` | string | `""` | Script to run after install |

## Examples

### Basic Application

```nix
buildNopherGoApp {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
}
```

### Multiple Binaries

```nix
buildNopherGoApp {
  pname = "myproject";
  version = "2.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
  subPackages = [
    "./cmd/server"
    "./cmd/client"
    "./cmd/cli"
  ];
}
```

### With Linker Flags

Embed version information at build time:

```nix
buildNopherGoApp {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
  ldflags = [
    "-s" "-w"  # Strip debug info
    "-X main.version=1.0.0"
    "-X main.commit=${self.shortRev or "dirty"}"
  ];
}
```

### With Build Tags

```nix
buildNopherGoApp {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
  tags = [ "production" "postgres" ];
}
```

### Cross-Compilation

Build for Linux from macOS:

```nix
buildNopherGoApp {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
  GOOS = "linux";
  GOARCH = "amd64";
}
```

Build for multiple platforms:

```nix
let
  buildFor = goos: goarch: buildNopherGoApp {
    pname = "myapp-${goos}-${goarch}";
    version = "1.0.0";
    src = ./.;
    modules = ./nopher.lock.yaml;
    GOOS = goos;
    GOARCH = goarch;
  };
in {
  linux-amd64 = buildFor "linux" "amd64";
  linux-arm64 = buildFor "linux" "arm64";
  darwin-amd64 = buildFor "darwin" "amd64";
  darwin-arm64 = buildFor "darwin" "arm64";
  windows-amd64 = buildFor "windows" "amd64";
}
```

### With CGO

```nix
buildNopherGoApp {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
  CGO_ENABLED = "1";
  nativeBuildInputs = [ pkg-config ];
  buildInputs = [ sqlite ];
}
```

### Post-Install Hook

Generate shell completions:

```nix
buildNopherGoApp {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
  postInstall = ''
    mkdir -p $out/share/bash-completion/completions
    $out/bin/myapp completion bash > $out/share/bash-completion/completions/myapp

    mkdir -p $out/share/zsh/site-functions
    $out/bin/myapp completion zsh > $out/share/zsh/site-functions/_myapp
  '';
}
```

### With Metadata

```nix
buildNopherGoApp {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./nopher.lock.yaml;
  meta = {
    description = "My awesome application";
    homepage = "https://github.com/myorg/myapp";
    license = lib.licenses.mit;
    maintainers = with lib.maintainers; [ yourname ];
    mainProgram = "myapp";
  };
}
```

## Integration with Flakes

### Full Flake Example

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    nopher.url = "github:anthr76/nopher";
  };

  outputs = { self, nixpkgs, flake-utils, nopher }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        nopherLib = nopher.lib.${system};
      in {
        packages = {
          default = nopherLib.buildNopherGoApp {
            pname = "myapp";
            version = "1.0.0";
            src = ./.;
            modules = ./nopher.lock.yaml;
          };
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            nopher.packages.${system}.default
          ];
        };
      }
    );
}
```

## Using the Overlay

Nopher provides a nixpkgs overlay:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    nopher.url = "github:anthr76/nopher";
  };

  outputs = { self, nixpkgs, nopher }:
    let
      pkgs = import nixpkgs {
        system = "x86_64-linux";
        overlays = [ nopher.overlays.default ];
      };
    in {
      packages.x86_64-linux.default = pkgs.buildNopherGoApp {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        modules = ./nopher.lock.yaml;
      };
    };
}
```
