{
  pkgs,
  lib,
  ...
}:

let
  isCI = builtins.getEnv "GITHUB_ACTIONS" != "";

  default = {
    containers = lib.mkForce { };

    cachix.enable = false;

    scripts = {
      lint-ci = {
        exec = ''
          modernize ./...
          # govulncheck ./...
        '';
      };
      lint-all = {
        exec = ''
          lint-ci
          golangci-lint run ./...
        '';
      };
      lint-fix = {
        exec = ''
          modernize --fix ./...
          golangci-lint run --fix ./...
        '';
      };
    };

    enterTest = ''
      go test -race ./...
    '';

    packages = with pkgs; [
      nodejs
      mockgen
      typos

      gcc
      gotools
      gopls
      govulncheck
      golangci-lint
    ];

    languages.go = {
      enable = true;
      version = "1.26.1";
    };

    env.GOTOOLCHAIN = lib.mkForce "local";
    env.GOFUMPT_SPLIT_LONG_LINES = lib.mkForce "on";
  };

  hooks = {
    git-hooks = {
      hooks = {
        lint = {
          enable = true;
          name = "lint";
          description = "Lint";
          entry = ''
            lint-all
          '';
          pass_filenames = false;
        };
      };
    };
  };
in

default // (if isCI then { } else hooks)
