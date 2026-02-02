{
  description = "Nopher - Custom Nix Go Module Builder with private repo support";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Import nopher library with explicit Go version
        # Use latest Go 1.25.x available
        nopher = import ./nix/default.nix {
          inherit pkgs;
          go = pkgs.go_1_25.overrideAttrs (oldAttrs: rec {
            version = "1.25.6";
            src = pkgs.fetchurl {
              url = "https://go.dev/dl/go${version}.src.tar.gz";
              hash = "sha256-WMv3ceRNdt5vVtGeM7d9dFoeSJNAkih15GWFuXXCsFk=";
            };
          });
        };

        # Build nopher using buildNopherGoApp (dogfooding)
        nopherCli = nopher.buildNopherGoApp {
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

      in
      {
        packages = {
          default = nopherCli;
          nopher = nopherCli;
        };

        # Export the nopher library for use in other flakes
        lib = nopher;

        apps.default = {
          type = "app";
          program = "${nopherCli}/bin/nopher";
        };

        # Overlay for use with nixpkgs
        overlays.default = import ./nix/overlay.nix;
      }
    ) // {
      # System-independent outputs
      overlays.default = import ./nix/overlay.nix;
    };
}
