{
  description = "LangLang: a parsing expression grammar library";

  inputs = {
    # Fork of upstream nixpkgs. overlays.default exposes buildGoApplication,
    # gomod2nix, and other amarbel-llc additions, so we don't need a
    # standalone gomod2nix flake input.
    igloo.url = "https://code.linenisgreat.com/igloo/archive/master.tar.gz";
    igloo.inputs.nixpkgs-master.follows = "nixpkgs-master";
    nixpkgs-master.url = "github:NixOS/nixpkgs/567a49d1913ce81ac6e9582e3553dd90a955875f";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    utils.inputs.systems.follows = "igloo/systems";
    bats = {
      url = "https://code.linenisgreat.com/bats/archive/master.tar.gz";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
    tap = {
      url = "https://code.linenisgreat.com/tap/archive/master.tar.gz";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
    tap.inputs.bats.follows = "bats";
    tap.inputs.purse-first.inputs.conformist.follows = "conformist";
    tap.inputs.treefmt-nix.follows = "igloo/treefmt-nix";

    # conformist: the linter/formatter multiplexer. `nix fmt` entry point;
    # config lives in ./conformist.nix (+ conformist.lib.presets.{eng,eng-go,
    # eng-impure} in outputs below). No igloo follow needed: conformist#93
    # (v0.1.19) made the impure lane's gomod2nix linter degrade gracefully
    # without igloo's overlay — and our own `pkgs` already carries it anyway.
    conformist = {
      url = "https://code.linenisgreat.com/conformist/archive/master.tar.gz";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
    conformist.inputs.igloo.follows = "igloo";
  };

  outputs =
    {
      self,
      igloo,
      nixpkgs-master,
      utils,
      bats,
      tap,
      conformist,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs-master = import nixpkgs-master { inherit system; };
        pkgs = import igloo {
          inherit system;
          overlays = [ igloo.overlays.default ];
        };

        conformistPkg = conformist.packages.${system}.default;

        # Pure lane: the eng presets (the eng-convention linters + the
        # canonical goimports->gofumpt Go formatter chain) plus this repo's
        # overlay (./conformist.nix). Drives `nix fmt` (build.wrapper), the
        # sandboxed checks.formatting (build.check), and the
        # conformist-pre-commit hook (build.preCommit).
        conformistEval = conformist.lib.evalModule pkgs {
          imports = [
            conformist.lib.presets.eng
            conformist.lib.presets.eng-go
            ./conformist.nix
          ];
          package = conformistPkg;
        };

        # Impure lane: the git-state eng-convention checks (git-remotes,
        # git-default-branch, sweatfile, agents-md, gomod2nix) that need a
        # live .git / real go.mod resolution, so they can't run in the
        # sandboxed checks.formatting. Runs against the working tree via
        # `just lint-worktree`, consuming packages.conformist-impure-config
        # below.
        conformistImpureEval = conformist.lib.evalModule pkgs {
          imports = [ conformist.lib.presets.eng-impure ];
          package = conformistPkg;
          projectRootFile = "flake.nix";
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

          # Store-pinned conformist hooks + configs (conformist#51/#59): the
          # per-commit restage hook, its merge-repair sibling, and the
          # generated impure-lane config consumed by `just lint-worktree`.
          conformist-pre-commit = conformistEval.config.build.preCommit;
          conformist-repair = conformistEval.config.build.repair;
          conformist-impure-config = conformistImpureEval.config.build.configFile;
          conformist = conformistPkg;
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
            conformistPkg
            conformistEval.config.build.preCommit
            conformistEval.config.build.repair
          ];
        };

        # `nix fmt` — the generated conformist wrapper (config + every
        # formatter baked as /nix/store paths) across the worktree.
        formatter = conformistEval.config.build.wrapper;

        # `nix flake check` — read-only formatting + eng-convention gate
        # (sandbox-pure; the git-state lane is `just lint-worktree`).
        checks.formatting = conformistEval.config.build.check self;
      }
    );
}
