# Nopher - Nix Go Module Builder
#
# This module provides functions for building Go applications using
# a nopher lockfile (nopher.lock.yaml).
#
# Example usage in a flake:
#
#   {
#     inputs.nopher.url = "github:anthr76/nopher";
#
#     outputs = { self, nixpkgs, nopher }:
#       let
#         system = "x86_64-linux";
#         pkgs = nixpkgs.legacyPackages.${system};
#         nopherPkgs = nopher.lib.${system};
#       in {
#         packages.${system}.default = nopherPkgs.buildNopherGoApp {
#           pname = "myapp";
#           version = "1.0.0";
#           src = ./.;
#           modules = ./nopher.lock.yaml;
#         };
#       };
#   }

{ pkgs ? import <nixpkgs> { }
, lib ? pkgs.lib
, stdenv ? pkgs.stdenv
, stdenvNoCC ? pkgs.stdenvNoCC
, go ? pkgs.go
, yj ? pkgs.yj
, fetchurl ? pkgs.fetchurl
, unzip ? pkgs.unzip
}:

let
  nopherLib = import ./lib.nix { inherit lib; };

  fetchGoModule = pkgs.callPackage ./fetch-module.nix { };

  buildNopherGoApp = pkgs.callPackage ./build-nopher-go-app.nix {
    inherit fetchGoModule yj;
  };

in
{
  inherit fetchGoModule buildNopherGoApp;
  lib = nopherLib;
}
