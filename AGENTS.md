# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) and other coding
agents when working with code in this repository. `CLAUDE.md` is a symlink to
this file.

## What this repo is

Our **fork** of [langlang](https://github.com/clarete/langlang), a parsing
expression grammar (PEG) toolkit by Lincoln Clarete — see NOTICE for upstream
attribution. The repo carries the full upstream git history (no squash) plus:

- first-party Go subsystems under `go/`: `tomlcst/` (TOML->CST translation),
  `extract/` (typed tree extraction + arena codegen), `junction/` (SIMD-guided
  scanning), `binary/` (binary parser codegen);
- `docs/decisions/`, `docs/features/`, `docs/plans/`, `docs/references/` — our
  own ADR/FDR/plan/reference records (upstream ships none of these);
- the eng repository conventions (conformist, this justfile).

Only `go/` is Nix-built (`packages.default`, consumed by hyphence/papi/
cutting-garden, plus `go-pkgs`/`go-pkgs-test` producer outputs for downstream
Go-module bridging — the flake-input-go_mod protocol, see `go/gomod.nix`) — `rust/` and `js/` ship upstream's own Rust crates and JS/WASM
playground and are untouched by the fork (`rust/` has had exactly one commit
since the fork's inception, a directory move by the upstream author).
Formatting is scoped to what the fork owns: Go source under `go/` and the Nix
layer. `*.md`, `testdata/**` (parser test fixtures — the literal bytes are
what's under test), and `benchmarks/**` (a separate, wholly upstream go.mod
module) are excluded (conformist.nix) so upstream merges stay clean.

langlang is versionless: no git tags in its history, no CHANGELOG, and
`rust/*/Cargo.toml`'s `0.1.2` has never moved independently. eng-versioning is
explicitly disabled (see conformist.nix, langlang#28); eng consumes this repo
by tracking master, not a version.

## Build / test / lint

Justfile recipes are paved paths; the sweatfile's pre-merge hook runs `just`
(= `lint build test`).

- `just lint-fmt` — sandboxed conformist gate (builds `checks.formatting`).
- `just lint-worktree` — impure lane: git-state eng checks (git-remotes,
  git-default-branch, sweatfile, agents-md, gomod2nix).
- `just build` — `build-go` (host `go build`) + `build-generate` (`go
  generate` using the freshly built binary).
- `just test` — `test-go` (`go test -v ./...`).
- `just codemod-fmt` — `nix fmt` (rewrites the worktree).
- `just clean` — `go clean -cache -modcache`.
