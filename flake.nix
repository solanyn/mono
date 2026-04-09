{
  description = "solanyn/mono development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        pg = pkgs.postgresql_17.withPackages (p: [ p.postgis ]);
      in
      {
        devShells.default = pkgs.mkShell {
          packages = [
            pg
            pkgs.redis
            pkgs.go
            pkgs.bazelisk
            pkgs.goose
            pkgs.python312
            pkgs.nodejs_22
            pkgs.pnpm
          ];

          shellHook = ''
            export PGBIN="${pg}/bin"
            export REDIS_BIN="${pkgs.redis}/bin"
            export LC_ALL=C
          '';
        };

        packages = {
          postgresql = pg;
          redis = pkgs.redis;
        } // (if pkgs.stdenv.isDarwin then {
          scrib = pkgs.buildGoModule {
            pname = "scrib";
            version = "0.1.0";
            src = ./scrib;
            vendorHash = "sha256-VmXvT/o04hJWLSFCzsSULav3MPVfHvU0LyIiEo407gI=";
            buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin [
              pkgs.apple-sdk_15
              (pkgs.darwinMinVersionHook "13.0")
            ];
            env.CGO_ENABLED = "1";
            meta = {
              description = "Meeting audio capture & annotation";
              platforms = pkgs.lib.platforms.darwin;
            };
          };
        } else {}) // {
          scrib-server = pkgs.buildGoModule {
            pname = "scrib-server";
            version = "0.1.0";
            src = ./scrib;
            subPackages = [ "cmd/scrib-server" ];
            vendorHash = "sha256-1LdrUGrOSvMPMItC8VhNiBpoB1Gq9nS4dMWYb4IgV7c=";
            env.CGO_ENABLED = "0";
            meta.description = "Scrib sync server";
          };
        };
      }
    );
}
