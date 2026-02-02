# fetchGoModule - Fetch a single Go module by path and version
#
# This function fetches a Go module from proxy.golang.org and extracts it.
# The hash should be a Nix SRI hash (e.g., "sha256-...") computed by nopher.
#
# Works with any Go module including:
# - Standard modules (github.com/*, golang.org/*, etc.)
# - BSR generated modules (buf.build/gen/go/*)
# - Self-hosted registries
#
# Usage:
#   fetchGoModule {
#     modulePath = "github.com/sirupsen/logrus";
#     version = "v1.9.3";
#     hash = "sha256-E5GnOMrWPCJLof4UFRJ9sLQKLpALbstsrqHmnWpnn5w=";
#   }

{ lib
, stdenvNoCC
, fetchurl
, unzip
}:

{ modulePath
, version
, hash
, # Optional: override the proxy URL
  proxy ? "https://proxy.golang.org"
}:

let
  nopherLib = import ./lib.nix { inherit lib; };

  # Escape the module path for the URL
  escapedPath = nopherLib.escapeModulePath modulePath;

  # Build the download URL
  url = "${proxy}/${escapedPath}/@v/${version}.zip";

  # Create a valid derivation name
  pname = nopherLib.modulePathToName modulePath;
in
stdenvNoCC.mkDerivation {
  name = "${pname}-${version}";
  inherit pname version;

  src = fetchurl {
    inherit url hash;
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

  # Disable fixup phases that might modify the output
  dontFixup = true;

  passthru = {
    inherit modulePath version;
  };

  meta = {
    description = "Go module ${modulePath} version ${version}";
    homepage = "https://pkg.go.dev/${modulePath}";
  };
}
