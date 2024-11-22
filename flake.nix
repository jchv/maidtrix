{
  description = "Open source Matrix homeserver.";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        dendrite = pkgs.buildGoModule {
          name = "dendrite";
          src = self;
          vendorHash = "sha256-P+F3TkA8627GlXeNg1PTwvpQ+xQYxUk2px0lyqQFV+Q=";
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
          inherit dendrite format;
          default = dendrite;
        };
        devShell = pkgs.mkShell {
          inputsFrom = [ dendrite ];
          nativeBuildInputs = [
            pkgs.go
            pkgs.gopls
          ];
        };
      }
    );
}
