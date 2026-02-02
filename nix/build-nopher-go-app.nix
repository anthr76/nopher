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
}:

{ pname
, version
, src
, # Path to nopher.lock.yaml
  modules
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
      fetchGoModule {
        modulePath = path;
        version = info.version;
        hash = info.hash;
      })
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

  # Build the vendor directory
  vendorDir = stdenv.mkDerivation {
    name = "${pname}-vendor";

    dontUnpack = true;
    dontBuild = true;

    installPhase = ''
      mkdir -p $out

      # Link regular modules
      ${lib.concatStringsSep "\n" (lib.mapAttrsToList (path: drv: ''
        mkdir -p $out/${nopherLib.dirOf path}
        ln -s ${drv} $out/${path}
      '') fetchedModules)}

      # Link replacement modules (non-local)
      ${lib.concatStringsSep "\n" (lib.mapAttrsToList (origPath: drv:
        if drv != null then ''
          mkdir -p $out/${nopherLib.dirOf origPath}
          # Remove existing link if any (from regular modules)
          rm -f $out/${origPath}
          ln -s ${drv} $out/${origPath}
        '' else ""
      ) fetchedReplaces)}

      # Create modules.txt for Go's vendor detection
      # Need to scan each module directory for all packages
      # Include go version so modules don't default to go1.16
      {
        ${lib.concatStringsSep "\n" (lib.mapAttrsToList (path: info: ''
          echo "# ${path} ${info.version}"
          echo "## explicit; go ${lockfileJson.go}"
          # Find all Go packages in this module (follow symlinks)
          find -L $out/${path} -name '*.go' -print0 2>/dev/null | xargs -0 -n1 dirname 2>/dev/null | sort -u | while read -r pkg_dir; do
            pkg_path="''${pkg_dir#$out/}"
            echo "$pkg_path"
          done
        '') (lockfileJson.modules or {}))}
      } > $out/modules.txt
    '';
  };

  # Build local replace paths for linking
  localReplaces = lib.filterAttrs (path: info: info ? path) (lockfileJson.replace or { });

  # Remove our custom attributes before passing to mkDerivation
  extraArgs = builtins.removeAttrs args [
    "pname"
    "version"
    "src"
    "modules"
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

  nativeBuildInputs = [ go ] ++ (args.nativeBuildInputs or [ ]);

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
