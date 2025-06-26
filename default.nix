let
  pkgs = import (builtins.fetchTree {
    type = "git";
    url = "https://github.com/nixos/nixpkgs/";
    rev = "d74a2335ac9c133d6bbec9fc98d91a77f1604c1f"; # 17-02-2025
    shallow = true;
    # obtain via `git ls-remote https://github.com/nixos/nixpkgs nixos-unstable`
  }) { config = {}; };

  # helpers
  pythonDevPkgs = python-packages: devDeps python-packages ++ appDeps python-packages;
  pythonAppPkgs = python-packages: appDeps python-packages;
  devPython = pythonCore.withPackages pythonDevPkgs;
  appPython = pythonCore.withPackages pythonAppPkgs;

  pythonCore = pkgs.python311;
  devDeps = p: with p; [
    ptpython # nicer repl
    pytest
  ];
  appDeps = p: with p; [
    # TODO: deps go here
  ];
in
rec {
  pname = "fillthis";
  version = "0.0.1";
  artifacts = rec {
    app = appPython.pkgs.buildPythonApplication {
      inherit pname version;

      src = builtins.filterSource (path: type:  baseNameOf path != ".git") ./.;

      dependencies = appDeps appPython.pkgs;
      # disable tests while building
      #dontUseSetuptoolsCheck = true;
    };
    container = pkgs.dockerTools.buildLayeredImage {
      name = pname;
      tag = version;

      #created = "now";

      contents = [ appPython app ];

      config = {
        Entrypoint = [
          "${app}/bin/main.py"
        ];
      };
    };
  };
  shell = pkgs.mkShellNoCC {
    packages = with pkgs; [
      git
      gnumake

      go
    ];
  };
}
