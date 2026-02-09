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

        go_1_25_6 = pkgs.go_1_25.overrideAttrs (oldAttrs: rec {
          version = "1.25.6";
          src = pkgs.fetchurl {
            url = "https://go.dev/dl/go${version}.src.tar.gz";
            hash = "sha256-WMv3ceRNdt5vVtGeM7d9dFoeSJNAkih15GWFuXXCsFk=";
          };
        });

        nopher = import ./nix/default.nix {
          inherit pkgs;
          go = go_1_25_6;
        };

        nopherCli = nopher.buildNopherGoApp {
          go = go_1_25_6;
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

        overlays.default = import ./nix/overlay.nix;
      }
    ) // { 
      overlays.default = import ./nix/overlay.nix;
    };
}
