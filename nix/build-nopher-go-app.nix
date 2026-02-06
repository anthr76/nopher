# buildNopherGoApp - Build a Go application using a nopher lockfile
#
# This function builds a Go application using pre-fetched dependencies
# specified in a nopher.lock.yaml file. Each dependency is fetched
# individually (enabling fine-grained caching) and assembled into a
# vendor directory using symlinks.
#
# Usage:
#   buildNopherGoApp {
#     pname = "myapp";
#     version = "1.0.0";
#     src = ./.;
#     modules = ./nopher.lock.yaml;
#   }

{ lib
, stdenv
, go
, yj
, fetchGoModule
, pkgs
}:

let
  # Preserve the default go from outer scope
  defaultGo = go;
in

{ pname
, version
, src
, # Path to nopher.lock.yaml
  modules
, # Go compiler (optional override)
  go ? defaultGo
, # Build options
  ldflags ? [ ]
, tags ? [ ]
, subPackages ? [ "." ]
, # Go environment
  CGO_ENABLED ? "0"
, GOOS ? null
, GOARCH ? null
, # Hooks
  preBuild ? ""
, postBuild ? ""
, preInstall ? ""
, postInstall ? ""
, # Extra attributes to pass to mkDerivation
  ...
} @ args:

let
  nopherLib = import ./lib.nix { inherit lib; };

  # Convert YAML lockfile to JSON at eval time using IFD
  # This is necessary because Nix doesn't natively parse YAML
  lockfileJson = builtins.fromJSON (builtins.readFile (
    stdenv.mkDerivation {
      name = "lockfile-json";
      nativeBuildInputs = [ yj ];
      buildCommand = ''
        yj -yj < ${modules} > $out
      '';
    }
  ));

  # Fetch each module
  fetchedModules = lib.mapAttrs
    (path: info:
      fetchGoModule ({
        modulePath = path;
        version = info.version;
        hash = info.hash;
      } // lib.optionalAttrs (info ? url) {
        url = info.url;
      } // lib.optionalAttrs (info ? rev) {
        rev = info.rev;
      }))
    (lockfileJson.modules or { });

  # Fetch replacement modules
  fetchedReplaces = lib.mapAttrs
    (path: info:
      if info ? path then
        # Local replacement - will be handled separately
        null
      else
        fetchGoModule {
          modulePath = info.new;
          version = info.version;
          hash = info.hash;
        })
    (lockfileJson.replace or { });

  # Determine which module paths have children
  # A path has children if another path starts with "path/"
  modulePathsWithChildren = lib.filter
    (path: lib.any (other: other != path && lib.hasPrefix (path + "/") other) (lib.attrNames fetchedModules))
    (lib.attrNames fetchedModules);

  # Build module path -> derivation mapping as JSON for runtime lookup
  moduleMapping = builtins.toJSON (lib.mapAttrs (path: drv: {
    store = "${drv}";
    hasChildren = lib.elem path modulePathsWithChildren;
  }) fetchedModules);

  replaceMapping = builtins.toJSON (lib.mapAttrs (origPath: drv:
    if drv != null then "${drv}" else null
  ) fetchedReplaces);

  # Build the vendor directory
  vendorDir = stdenv.mkDerivation {
    name = "${pname}-vendor";

    dontUnpack = true;
    dontBuild = true;

    passAsFile = [ "moduleMapping" "replaceMapping" ];
    inherit moduleMapping replaceMapping;

    nativeBuildInputs = [ pkgs.jq ];

    installPhase = ''
      mkdir -p $out

      # Helper function to ensure parent directory exists
      ensure_parent() {
        local path="$1"
        local parent=$(dirname "$path")
        if [ "$parent" != "." ] && [ "$parent" != "$out" ]; then
          while [ -L "$out/$parent" ] && [ -e "$out/$parent" ]; do
            local target=$(readlink -f "$out/$parent")
            rm "$out/$parent"
            mkdir -p "$out/$parent"
            if [ -d "$target" ]; then
              cp -r "$target"/* "$out/$parent/" 2>/dev/null || true
            fi
          done
          mkdir -p "$out/$parent"
        fi
      }

      # Process modules from JSON mapping
      jq -r 'to_entries[] | "\(.key)|\(.value.store)|\(.value.hasChildren)"' < "$moduleMappingPath" | while IFS='|' read -r path store hasChildren; do
        parent=$(dirname "$out/$path")
        mkdir -p "$parent"

        if [ "$hasChildren" = "true" ]; then
          cp -r "$store" "$out/$path"
          chmod -R +w "$out/$path"
        else
          ln -s "$store" "$out/$path"
        fi
      done

      # Process replacements from JSON mapping
      jq -r 'to_entries[] | select(.value != null) | "\(.key)|\(.value)"' < "$replaceMappingPath" | while IFS='|' read -r origPath store; do
        mkdir -p "$out/$(dirname "$origPath")"
        rm -f "$out/$origPath"
        ln -s "$store" "$out/$origPath"
      done

      # Create modules.txt using a helper script
      # Generate modules.txt from the vendored modules
      bash ${builtins.toFile "gen-modules-txt.sh" (''
        #!/bin/bash
        set -e
        (
      '' + lib.concatStringsSep "\n" (lib.mapAttrsToList (path: info: ''
        echo "# ${path} ${info.version}"
        echo "## explicit; go ${lockfileJson.go}"
        find -L "$out/${path}" -name '*.go' -print0 2>/dev/null | xargs -0 -n1 dirname 2>/dev/null | sort -u | while read -r pkg_dir; do
          pkg_path="''${pkg_dir#$out/}"
          echo "$pkg_path"
        done
      '') (lockfileJson.modules or {})) + "\n" + lib.concatStringsSep "\n" (lib.mapAttrsToList (origPath: replaceInfo:
        if replaceInfo ? path then ""
        else
          let origVersion = replaceInfo.oldVersion or (lockfileJson.modules.${origPath} or {}).version or "v0.0.0";
          in ''
            echo "# ${origPath} ${origVersion} => ${replaceInfo.new} ${replaceInfo.version}"
            echo "## explicit; go ${lockfileJson.go}"
            find -L "$out/${origPath}" -name '*.go' -print0 2>/dev/null | xargs -0 -n1 dirname 2>/dev/null | sort -u | while read -r pkg_dir; do
              pkg_path="''${pkg_dir#$out/}"
              echo "$pkg_path"
            done
          ''
      ) (lockfileJson.replace or {})) + ''
        ) > "$out/modules.txt"
      '')}
    '';
  };

  # Build local replace paths for linking
  localReplaces = lib.filterAttrs (path: info: info ? path) (lockfileJson.replace or { });

  # Use provided go compiler
  goCompiler = go;

  # Remove our custom attributes before passing to mkDerivation
  extraArgs = builtins.removeAttrs args [
    "pname"
    "version"
    "src"
    "modules"
    "go"
    "ldflags"
    "tags"
    "subPackages"
    "CGO_ENABLED"
    "GOOS"
    "GOARCH"
    "preBuild"
    "postBuild"
    "preInstall"
    "postInstall"
  ];

in
stdenv.mkDerivation (extraArgs // {
  inherit pname version src;

  nativeBuildInputs = [ goCompiler ] ++ (args.nativeBuildInputs or [ ]);

  configurePhase = ''
    runHook preConfigure

    # Set up Go environment
    export HOME=$TMPDIR
    export GOCACHE=$TMPDIR/go-cache
    export GOPATH=$TMPDIR/go
    export GOMODCACHE=$TMPDIR/go-mod-cache
    export GO111MODULE=on
    export GOTOOLCHAIN=local
    export CGO_ENABLED=${CGO_ENABLED}
    ${lib.optionalString (GOOS != null) "export GOOS=${GOOS}"}
    ${lib.optionalString (GOARCH != null) "export GOARCH=${GOARCH}"}

    # Offline mode - no network access
    export GOPROXY=off
    export GOSUMDB=off

    # Copy vendor directory (we can't just symlink because Go doesn't like it)
    # Use -L to dereference symlinks and copy actual files
    cp -rL ${vendorDir} vendor
    chmod -R +w vendor

    # Remove go.mod files from vendor - Go uses modules.txt in vendor mode
    # and vendored go.mod files without a go directive default to go1.16
    # which causes compilation errors for code using newer features
    find vendor -name 'go.mod' -delete
    find vendor -name 'go.sum' -delete

    # Link local replacements from source
    ${lib.concatStringsSep "\n" (lib.mapAttrsToList (origPath: info: ''
      if [ -d "${info.path}" ]; then
        mkdir -p vendor/${nopherLib.dirOf origPath}
        rm -rf vendor/${origPath}
        cp -r "${info.path}" vendor/${origPath}
      fi
    '') localReplaces)}

    runHook postConfigure
  '';

  buildPhase = ''
    runHook preBuild
    ${preBuild}

    go build \
      -mod=vendor \
      -trimpath \
      ${lib.optionalString (ldflags != [ ]) "-ldflags '${lib.concatStringsSep " " ldflags}'"} \
      ${lib.optionalString (tags != [ ]) "-tags '${lib.concatStringsSep "," tags}'"} \
      -o $TMPDIR/bin/ \
      ${lib.concatStringsSep " " subPackages}

    ${postBuild}
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    ${preInstall}

    mkdir -p $out/bin
    cp -r $TMPDIR/bin/* $out/bin/

    ${postInstall}
    runHook postInstall
  '';

  meta = {
    description = args.meta.description or "Go application built with nopher";
  } // (args.meta or { });
})
