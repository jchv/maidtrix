{
  description = "Open source Matrix homeserver.";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
    sytest-src = {
      url = "github:matrix-org/sytest";
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      sytest-src,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        inherit (pkgs) perlPackages;
        maidtrix = pkgs.buildGoModule {
          name = "maidtrix";
          src = self;
          vendorHash = "sha256-P+F3TkA8627GlXeNg1PTwvpQ+xQYxUk2px0lyqQFV+Q=";
        };
        sytest = perlPackages.buildPerlPackage {
          pname = "sytest";
          version = "0-unstable";
          src = sytest-src;
          propagatedBuildInputs = [
            perlPackages.ClassMethodModifiers
            perlPackages.CryptEd25519
            perlPackages.DataDump
            perlPackages.DBI
            perlPackages.DBDPg
            perlPackages.DigestHMAC_SHA1
            perlPackages.DigestSHA
            perlPackages.EmailAddressXS
            perlPackages.EmailMIME
            perlPackages.FilePath
            perlPackages.FileSlurper
            perlPackages.Future
            perlPackages.GetoptLong
            perlPackages.IOAsync
            perlPackages.IOAsyncSSL
            perlPackages.IOSocketIP
            perlPackages.IOSocketSSL
            perlPackages.JSON
            perlPackages.JSONPP
            perlPackages.ListAllUtils
            perlPackages.MIMEBase64
            perlPackages.ModulePluggable
            perlPackages.NetAsyncHTTP
            perlPackages.NetAsyncHTTPServer
            perlPackages.NetSSLeay
            perlPackages.StructDumb
            perlPackages.URIEscapeXS
            perlPackages.YAML
          ];
        };
        format = pkgs.writeShellApplication {
          name = "format";

          runtimeInputs = [
            pkgs.nixfmt-rfc-style
            pkgs.yamlfmt
            pkgs.go
          ];

          text = ''
            shopt -s globstar
            GLOBIGNORE=".:.."

            if [[ $# -ne 1 || "$1" == "--help" ]]; then
              >&2 echo "Usage: $0 --check | --write"
              exit 0
            fi

            NIXFMT_ARGS=()
            YAMLFMT_ARGS=()

            case $1 in
              -w|--write)
                NIXFMT_ARGS+=("--verify")
                GOFMT_COMMAND="gofmt -w ."
                shift
                ;;
              -c|--check)
                NIXFMT_ARGS+=("--check")
                YAMLFMT_ARGS+=("-dry" "-lint")
                GOFMT_COMMAND="diff <(echo -n) <(gofmt -d .)"
                shift
                ;;
              *)
                >&2 echo "Unknown option $1"
                exit 1
                ;;
            esac

            set -x

            >&2 echo "Running nixfmt."
            find . -not -path '*/.*' -not -path 'build' -iname '*.nix' -print0 | \
              xargs -0 nixfmt "''${NIXFMT_ARGS[@]}"

            >&2 echo "Running yamlfmt."
            yamlfmt "''${YAMLFMT_ARGS[@]}" ./**/*.yaml

            >&2 echo "Running gofmt."
            bash -c "''${GOFMT_COMMAND}"
          '';
        };
      in
      {
        packages = {
          inherit maidtrix sytest format;
          default = maidtrix;
        };
        devShell = pkgs.mkShell {
          inputsFrom = [ maidtrix ];
          nativeBuildInputs = [
            pkgs.go
            pkgs.gopls
          ];
        };
      }
    );
}
