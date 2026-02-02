{ pkgs, lib, config, inputs, ... }:

let
  go_1_25_6 = pkgs.go_1_25.overrideAttrs (old: rec {
    version = "1.25.6";
    src = pkgs.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-WMv3ceRNdt5vVtGeM7d9dFoeSJNAkih15GWFuXXCsFk=";
    };
  });
in
{
  # Development packages
  packages = [
    pkgs.git
    pkgs.delve
  ];

  # Go language support - pin to 1.25.6
  languages.go = {
    enable = true;
    package = pkgs.go_1_25;
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
    generate.exec = "./bin/nopher generate";
    docs-dev.exec = "cd docs && npm install && npm start";
    docs-build.exec = "cd docs && npm install && npm run build";
  };

  enterShell = ''
    echo ""
    echo "Nopher development environment"
    echo ""
    echo "Available commands:"
    echo "  build       - Build nopher binary"
    echo "  test        - Run tests"
    echo "  generate    - Generate lockfile"
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
