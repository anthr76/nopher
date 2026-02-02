# Nixpkgs overlay for nopher
#
# This overlay adds nopher functions to nixpkgs.
#
# Usage:
#   let
#     pkgs = import nixpkgs {
#       overlays = [ (import ./nix/overlay.nix) ];
#     };
#   in
#     pkgs.nopher.buildNopherGoApp { ... }

final: prev:

let
  nopher = import ./default.nix {
    pkgs = final;
    lib = final.lib;
  };
in
{
  nopher = nopher;

  # Also expose at top-level for convenience
  fetchGoModule = nopher.fetchGoModule;
  buildNopherGoApp = nopher.buildNopherGoApp;
}
