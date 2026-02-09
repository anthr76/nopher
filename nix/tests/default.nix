{ pkgs ? import <nixpkgs> {} }:

let
  nopherLib = import ../lib.nix { inherit (pkgs) lib; };
in

{
  testModulePathToName = {
    expr = nopherLib.modulePathToName "github.com/foo/bar";
    expected = "github-com-foo-bar";
  };

  testModulePathToNameWithDots = {
    expr = nopherLib.modulePathToName "golang.org/x/sys";
    expected = "golang-org-x-sys";
  };

  testDirOf = {
    expr = nopherLib.dirOf "github.com/foo/bar";
    expected = "github.com/foo";
  };

  testDirOfRoot = {
    expr = nopherLib.dirOf "github.com";
    expected = ".";
  };

  testEscapeModulePath = {
    expr = nopherLib.escapeModulePath "github.com/Azure/go-autorest";
    expected = "github.com/!azure/go-autorest";
  };

  testEscapeModulePathLowercase = {
    expr = nopherLib.escapeModulePath "github.com/sirupsen/logrus";
    expected = "github.com/sirupsen/logrus";
  };

  testMatchesPrivateExact = {
    expr = nopherLib.matchesPrivate "github.com/myorg/repo" "github.com/myorg/repo";
    expected = true;
  };

  testMatchesPrivateWildcard = {
    expr = nopherLib.matchesPrivate "github.com/myorg/*" "github.com/myorg/private";
    expected = true;
  };

  testMatchesPrivatePrefix = {
    expr = nopherLib.matchesPrivate "github.com/myorg" "github.com/myorg/repo";
    expected = true;
  };

  testMatchesPrivateNoMatch = {
    expr = nopherLib.matchesPrivate "github.com/myorg/*" "github.com/other/repo";
    expected = false;
  };

  testIsPrivate = {
    expr = nopherLib.isPrivate ["github.com/myorg/*" "gitlab.com/internal/*"] "github.com/myorg/private";
    expected = true;
  };

  testIsPrivateMultiplePatterns = {
    expr = nopherLib.isPrivate ["github.com/myorg/*" "gitlab.com/internal/*"] "gitlab.com/internal/project";
    expected = true;
  };

  testIsPrivateNoMatch = {
    expr = nopherLib.isPrivate ["github.com/myorg/*"] "github.com/other/repo";
    expected = false;
  };
}
