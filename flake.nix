{
  description = "Nopher - Custom Nix Go Module Builder with private repo support";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    nix-unit.url = "github:nix-community/nix-unit";
    nix-unit.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, flake-utils, nix-unit }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        nopher = import ./nix/default.nix {
          inherit pkgs;
        };

        nopherCli = nopher.buildNopherGoApp {
          go = pkgs.go;
          pname = "nopher";
          version = "0.1.0";
          src = ./.;
          modules = ./nopher.lock.yaml;
          subPackages = [ "./cmd/nopher" ];

          meta = {
            description = "Generate Nix-compatible lockfiles from Go modules";
            homepage = "https://github.com/anthr76/nopher";
            license = pkgs.lib.licenses.mit;
            maintainers = [ ];
            mainProgram = "nopher";
          };
        };

        tests = import ./nix/tests/default.nix { inherit pkgs; };

      in
      {
        inherit tests;

        packages = {
          default = nopherCli;
          nopher = nopherCli;
        };

        lib = nopher;

        checks = {
          nix-unit = pkgs.runCommand "nix-unit-tests" {
            nativeBuildInputs = [ nix-unit.packages.${system}.default ];
          } ''
            export HOME=$TMPDIR
            nix-unit --flake ${self}#tests
            touch $out
          '';
        };

        apps.default = {
          type = "app";
          program = "${nopherCli}/bin/nopher";
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.gotools
            pkgs.go-tools
            pkgs.delve
            nix-unit.packages.${system}.default
            nopherCli
          ];
        };

        overlays.default = import ./nix/overlay.nix;
      }
    ) // { 
      overlays.default = import ./nix/overlay.nix;
    };
}
