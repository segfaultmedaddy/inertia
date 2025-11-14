{
  description = "inertia - Go adapter for Inertia.js";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    treefmt-nix.url = "github:numtide/treefmt-nix";
    git-hooks-nix.url = "github:cachix/git-hooks.nix";
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
        inputs.git-hooks-nix.flakeModule
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
              typos.enable = true;
            };
          };

          pre-commit = {
            settings.enabledPackages = with pkgs; [
              just
            ];

            settings.hooks = {
              lint = {
                enable = true;
                name = "lint";
                description = "Go Lint";
                entry = ''
                  just lint
                '';
                pass_filenames = false;
              };

              nixfmt.enable = true;
            };
          };

          devshells.default = {
            packages = with pkgs; [
              nodejs
              just
              mockgen
              gotools
              go
              gcc
              gopls
              govulncheck
              golangci-lint
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
