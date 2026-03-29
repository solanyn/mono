{
  description = "solanyn/mono development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
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
            pkgs.go_1_23
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
        };
      }
    );
}
