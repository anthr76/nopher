# Helper functions for nopher Nix builder
{ lib }:

rec {
  # Convert a module path to a valid Nix derivation name
  # e.g., "github.com/foo/bar" -> "github-com-foo-bar"
  modulePathToName = path:
    builtins.replaceStrings [ "/" "." ] [ "-" "-" ] path;

  # Get the directory component of a path
  # e.g., "github.com/foo/bar" -> "github.com/foo"
  dirOf = path:
    let
      parts = lib.splitString "/" path;
      init = lib.init parts;
    in
    if init == [ ] then "." else lib.concatStringsSep "/" init;

  # Escape a module path for use in Go proxy URLs
  # Go proxy encodes uppercase letters as !lowercase
  escapeModulePath = path:
    let
      chars = lib.stringToCharacters path;
      escapeChar = c:
        if c >= "A" && c <= "Z" then
          "!" + lib.toLower c
        else
          c;
    in
    lib.concatStrings (map escapeChar chars);

  # Parse the lockfile and return module info
  # Expects a JSON file path
  parseLockfile = lockfilePath:
    builtins.fromJSON (builtins.readFile lockfilePath);

  # Check if a module path matches a GOPRIVATE pattern
  matchesPrivate = pattern: path:
    if lib.hasSuffix "/*" pattern then
      let prefix = lib.removeSuffix "/*" pattern;
      in lib.hasPrefix (prefix + "/") path || path == prefix
    else if lib.hasSuffix "*" pattern then
      lib.hasPrefix (lib.removeSuffix "*" pattern) path
    else
      lib.hasPrefix pattern path;

  # Check if any pattern in a list matches
  isPrivate = patterns: path:
    lib.any (p: matchesPrivate p path) patterns;
}
