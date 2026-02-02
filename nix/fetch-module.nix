# fetchGoModule - Fetch a single Go module by path and version
#
# This function fetches a Go module using the appropriate method:
# - GitHub repos: Uses builtins.fetchGit (supports netrc authentication)
# - BSR modules: Uses fetchurlBoot
# - Other modules: Uses proxy.golang.org
#
# Usage:
#   fetchGoModule {
#     modulePath = "github.com/sirupsen/logrus";
#     version = "v1.9.3";
#     hash = "sha256-E5GnOMrWPCJLof4UFRJ9sLQKLpALbstsrqHmnWpnn5w=";
#     url = "https://github.com/sirupsen/logrus/archive/refs/tags/v1.9.3.zip";
#   }

{ lib
, stdenv
, stdenvNoCC
, fetchurl
, unzip
}:

{ modulePath
, version
, hash
, # Optional: explicit URL from lockfile (preferred)
  url ? null
, # Optional: git commit hash (for fetchGit)
  rev ? null
, # Optional: override the proxy URL (fallback)
  proxy ? "https://proxy.golang.org"
}:

let
  nopherLib = import ./lib.nix { inherit lib; };

  # Check if we have a GitHub archive URL with a full 40-char rev
  # fetchGit requires either a ref or a full 40-character rev to work in pure mode
  # If rev is missing or truncated, fall back to fetchurl
  hasFullRev = rev != null && (builtins.stringLength rev) == 40;
  isGitHubArchiveURL = url != null && hasFullRev && lib.hasPrefix "https://github.com/" url && lib.hasInfix "/archive/" url;

  # Parse GitHub URL to extract repo info and ref/rev
  # URL formats:
  # - https://github.com/owner/repo/archive/refs/tags/tagname.zip
  # - https://github.com/owner/repo/archive/commithash.zip
  parseGitHubURL = url:
    let
      # Remove https://github.com/ prefix
      withoutPrefix = lib.removePrefix "https://github.com/" url;
      # Split by /
      parts = lib.splitString "/" withoutPrefix;
      owner = lib.elemAt parts 0;
      repo = lib.elemAt parts 1;
      # Check if it's a tag or commit hash
      archivePart = lib.concatStringsSep "/" (lib.drop 3 parts);
      # Remove .zip suffix
      withoutZip = lib.removeSuffix ".zip" archivePart;

      isTag = lib.hasPrefix "refs/tags/" withoutZip;
      isCommit = !isTag;

      ref = if isTag then withoutZip else null;
      rev = if isCommit then withoutZip else null;
    in
    {
      inherit owner repo ref rev;
      repoUrl = "https://github.com/${owner}/${repo}";
    };

  # For GitHub modules with archive URLs, use fetchGit which supports netrc
  githubSrc =
    if isGitHubArchiveURL then
      let
        parsed = parseGitHubURL url;
        # Extract subdir if the module path has more than owner/repo
        pathParts = lib.splitString "/" modulePath;
        subdir = if (lib.length pathParts) > 3
                 then lib.concatStringsSep "/" (lib.drop 3 pathParts)
                 else null;

        # If rev is short (< 40 chars), it's truncated and we can't use it with fetchGit
        # In that case, fall back to using ref only
        revLength = builtins.stringLength rev;
        useRev = revLength == 40;
      in
      builtins.fetchGit (
        { url = parsed.repoUrl;
          allRefs = true;
        }
        // lib.optionalAttrs (parsed.ref != null) { ref = parsed.ref; }
        // lib.optionalAttrs useRev { inherit rev; }  # Use rev only if it's full 40-char hash
      )
    else null;

  # For non-GitHub modules, use fetchurlBoot
  # Escape the module path for the URL
  escapedPath = nopherLib.escapeModulePath modulePath;

  # For BSR modules, we need the full path in the URL
  isBSR = lib.hasInfix "/gen/go/" modulePath;

  # Build the download URL for non-GitHub modules
  downloadURL =
    if url != null then
      url
    else if isBSR then
      let
        host = lib.head (lib.splitString "/" modulePath);
      in
        "https://${host}/gen/go/${escapedPath}/@v/${version}.zip"
    else
      "${proxy}/${escapedPath}/@v/${version}.zip";

  # Create a valid derivation name
  pname = nopherLib.modulePathToName modulePath;
in
# For GitHub repos, extract the module from the git checkout
if githubSrc != null then
  stdenvNoCC.mkDerivation {
    name = "${pname}-${version}";
    inherit pname version;

    src = githubSrc;

    dontBuild = true;
    dontConfigure = true;

    installPhase = ''
      runHook preInstall

      mkdir -p $out

      # Extract subdir if needed
      ${let
        pathParts = lib.splitString "/" modulePath;
        subdir = if (lib.length pathParts) > 3
                 then lib.concatStringsSep "/" (lib.drop 3 pathParts)
                 else "";
      in
        if subdir != "" then ''
          if [ -d "${subdir}" ]; then
            shopt -s dotglob
            cp -r ${subdir}/* $out/
            shopt -u dotglob
          else
            shopt -s dotglob
            cp -r * $out/
            shopt -u dotglob
          fi
        '' else ''
          shopt -s dotglob
          cp -r * $out/
          shopt -u dotglob
        ''
      }

      runHook postInstall
    '';

    dontFixup = true;

    passthru = {
      inherit modulePath version;
    };

    meta = {
      description = "Go module ${modulePath} version ${version}";
      homepage = "https://pkg.go.dev/${modulePath}";
    };
  }
else
  # For non-GitHub modules
  # Use builtins.fetchurl for BSR (supports netrc via netrc-file config)
  # Use fetchurlBoot for others
  stdenvNoCC.mkDerivation {
    name = "${pname}-${version}";
    inherit pname version;

    src = if isBSR then
      builtins.fetchurl {
        url = downloadURL;
        sha256 = lib.removePrefix "sha256-" hash;
      }
    else
      stdenv.fetchurlBoot {
        url = downloadURL;
        inherit hash;
      };

    nativeBuildInputs = [ unzip ];

    sourceRoot = ".";

    unpackPhase = ''
      runHook preUnpack
      unzip -q $src
      runHook postUnpack
    '';

    installPhase = ''
      runHook preInstall

      # The zip contains files under modulePath@version/
      # Move the contents to $out
      if [ -d "${modulePath}@${version}" ]; then
        mv "${modulePath}@${version}" $out
      else
        # Handle case where directory structure is different
        mkdir -p $out
        # Find the extracted directory and move its contents
        for dir in */; do
          if [ -d "$dir" ]; then
            cp -r "$dir"* $out/ 2>/dev/null || mv "$dir" $out/
            break
          fi
        done
      fi

      runHook postInstall
    '';

    dontFixup = true;

    passthru = {
      inherit modulePath version;
    };

    meta = {
      description = "Go module ${modulePath} version ${version}";
      homepage = "https://pkg.go.dev/${modulePath}";
    };
  }
