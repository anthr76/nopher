{ pkgs, lib, config, inputs, ... }:

let
  go_1_25_6 = pkgs.go_1_25.overrideAttrs (old: rec {
    version = "1.25.6";
    src = pkgs.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-WMv3ceRNdt5vVtGeM7d9dFoeSJNAkih15GWFuXXCsFk=";
    };
  });

  # Build nopher for development
  nopher = import ./nix/default.nix {
    inherit pkgs;
    go = go_1_25_6;
  };

  nopherCli = nopher.buildNopherGoApp {
    pname = "nopher";
    version = "0.1.0-dev";
    go = go_1_25_6;
    src = ./.;
    modules = ./nopher.lock.yaml;
    subPackages = [ "./cmd/nopher" ];
  };
in
{
  # Development packages
  packages = [
    pkgs.git
    pkgs.delve
    nopherCli
  ];
  languages.go = {
    enable = true;
    enableHardeningWorkaround = true;
    package = go_1_25_6;
  };

  languages.javascript.enable = true;
  languages.javascript.npm.enable = true;

  # Pre-commit hooks
  git-hooks.hooks = {
    gofmt.enable = true;
    govet.enable = true;
  };

  # Scripts
  scripts = {
    build.exec = "go build -o ./bin/nopher ./cmd/nopher";
    test.exec = "go test ./...";
    docs-dev.exec = "cd docs && npm install && npm start";
    docs-build.exec = "cd docs && npm install && npm run build";
  };

  enterShell = ''
    echo ""
    echo "Nopher development environment"
    echo ""
    echo "Available commands:"
    echo "  nopher      - Run nopher CLI ($(command -v nopher 2>/dev/null || echo 'not in PATH'))"
    echo "  build       - Build nopher from source"
    echo "  test        - Run tests"
    echo "  docs-dev    - Start docs dev server"
    echo "  docs-build  - Build docs for production"
    echo ""
  '';

  enterTest = ''
    echo "Running tests"
    go version
    go test ./...
  '';
}
