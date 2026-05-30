{
  description = "LangLang: a parsing expression grammar library";

  inputs = {
    # Fork of upstream nixpkgs. overlays.default exposes buildGoApplication,
    # gomod2nix, and other amarbel-llc additions, so we don't need a
    # standalone gomod2nix flake input.
    igloo.url = "github:amarbel-llc/igloo";
    nixpkgs-master.url = "github:NixOS/nixpkgs/d233902339c02a9c334e7e593de68855ad26c4cb";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    bats = {
      url = "github:amarbel-llc/bats";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
    tap = {
      url = "github:amarbel-llc/tap";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
  };

  outputs =
    {
      self,
      igloo,
      nixpkgs-master,
      utils,
      bats,
      tap,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs-master = import nixpkgs-master { inherit system; };
        pkgs = import igloo {
          inherit system;
          overlays = [ igloo.overlays.default ];
        };
      in
      {
        packages = {
          default = pkgs.buildGoApplication {
            pname = "langlang";
            version = "0.1.0";
            src = ./go;
            modules = ./go/gomod2nix.toml;
            subPackages = [ "cmd/langlang" ];
            go = pkgs-master.go;

            meta = {
              description = "A parsing expression grammar library";
              homepage = "https://github.com/clarete/langlang";
              license = pkgs.lib.licenses.gpl3Only;
            };
          };
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs-master.go
            pkgs-master.gopls
            pkgs-master.gotools
            pkgs-master.golangci-lint
            pkgs-master.delve
            pkgs-master.nixfmt
            pkgs.gomod2nix
            pkgs.just
            bats.packages.${system}.batman
            bats.packages.${system}.bats
            tap.packages.${system}.tap-dancer
          ];
        };
      }
    );
}
