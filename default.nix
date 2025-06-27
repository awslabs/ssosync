let
  pkgs = import (builtins.fetchTree {
    type = "git";
    url = "https://github.com/nixos/nixpkgs/";
    rev = "d74a2335ac9c133d6bbec9fc98d91a77f1604c1f"; # 17-02-2025
    shallow = true;
    # obtain via `git ls-remote https://github.com/nixos/nixpkgs nixos-unstable`
  }) { config = {}; };
in
rec {
  pname = "fillthis";
  version = "0.0.1";
  #artifacts = rec {
    #app = ...;
  #};
  shell = pkgs.mkShellNoCC {
    packages = with pkgs; [
      git
      gnumake

      go

      gopls
    ];
  };
}
