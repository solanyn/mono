{
  description = "Goyangi cross-compilation environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        pkgsCross = pkgs.pkgsCross;
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            bazel_7
            gcc
            rustc
            cargo
          ];
        };

        # Cross-compilation packages
        packages = {
          # Linux x86_64 toolchain
          gcc-x86_64-linux = pkgsCross.gnu64.stdenv.cc;
          rust-x86_64-linux = pkgsCross.gnu64.rust.packages.stable.rustPlatform.rust.rustc;
          
          # Linux ARM64 toolchain  
          gcc-aarch64-linux = pkgsCross.aarch64-multiplatform.stdenv.cc;
          rust-aarch64-linux = pkgsCross.aarch64-multiplatform.rust.packages.stable.rustPlatform.rust.rustc;
        };

        # Configuration for Bazel
        bazel = {
          nixpkgs = pkgs;
          gcc-x86_64-linux = pkgsCross.gnu64.stdenv.cc;
          gcc-aarch64-linux = pkgsCross.aarch64-multiplatform.stdenv.cc;
          rust-x86_64-linux = pkgsCross.gnu64.rust.packages.stable.rustPlatform.rust.rustc;
          rust-aarch64-linux = pkgsCross.aarch64-multiplatform.rust.packages.stable.rustPlatform.rust.rustc;
        };
      });
}