{
  description = "inertia - Go adapter for Inertia.js";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    treefmt-nix.url = "github:numtide/treefmt-nix";
    flake-root.url = "github:srid/flake-root";
    flake-parts.url = "github:hercules-ci/flake-parts";
    devshell.url = "github:numtide/devshell";
  };

  outputs =
    {
      self,
      flake-parts,
      ...
    }@inputs:
    flake-parts.lib.mkFlake { inherit inputs; } {
      flake = { };

      systems = [
        "x86_64-linux"
        "x86_64-darwin"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      imports = [
        inputs.devshell.flakeModule
        inputs.treefmt-nix.flakeModule
        inputs.flake-root.flakeModule
      ];

      perSystem =
        {
          pkgs,
          config,
          ...
        }:
        {
          formatter = config.treefmt.build.wrapper;

          treefmt.config = {
            inherit (config.flake-root) projectRootFile;
            package = pkgs.treefmt;

            programs = {
              nixfmt.enable = true;
              gofumpt.enable = true;
            };
          };

          devshells.default = {
            packages = with pkgs; [
              watchexec
              just
              lefthook
              typos

              go_1_25
              gotools
              gcc
              golangci-lint

              nodejs
            ];

            env = [
              {
                name = "GOTOOLCHAIN";
                value = "local";
              }
              {
                name = "GOFUMPT_SPLIT_LONG_LINES";
                value = "on";
              }
            ];
          };
        };
    };
}
