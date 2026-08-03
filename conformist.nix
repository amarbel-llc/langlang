# langlang's conformist overlay, merged with conformist.lib.presets.{eng,eng-go}
# in flake.nix (conformist.lib.evalModule). presets.eng enables the
# eng-convention linters (eng-versioning, flake-outputs/lock, the justfile-*
# roster); presets.eng-go carries the canonical goimports -> gofumpt chain for
# Go sources. Here live nixfmt, the versionless eng-versioning opt-out, and
# the fork-churn excludes.
{ lib, ... }:
{
  programs.nixfmt.enable = true;

  # go.mod lives at go/, not the tree root (rust/ and js/ are siblings, each
  # with their own build). Without this, goimports/gofumpt run with cwd at
  # the tree root, where Go tooling can't resolve the module — confirmed by
  # hand: `goimports -d tests/java8/java8_test.go` from the tree root
  # DELETES the (correctly used) unaliased `"code.linenisgreat.com/langlang/go"`
  # import as apparently-unused (it can't discover the import provides the
  # `langlang` identifier — the package's declared name doesn't match its
  # path's last segment, "go"), while `cd go && goimports -d ...` correctly
  # ADDS the explicit `langlang "code.linenisgreat.com/langlang/go"` alias
  # instead. That's a silent build break, not a style nit. workingDir
  # (conformist#38) scopes the formatter's cwd to go/, matching the working
  # `cd go &&` invocation.
  programs.goimports.workingDir = "go";
  programs.gofumpt.workingDir = "go";

  # langlang is versionless by design: zero git tags across 1089 commits (both
  # upstream clarete/langlang and this fork's own history), no CHANGELOG, and
  # rust/*/Cargo.toml's 0.1.2 is upstream's vestigial crates.io version that
  # has never moved independently (rust/ has had exactly one commit since the
  # fork's inception — a directory move, by the upstream author). eng consumes
  # this repo by tracking master (the archive/master.tar.gz flake input papi/
  # hyphence/cutting-garden declare), never by version or tag.
  #
  # conformist v0.1.19 fixed the eng-versioning trigger gate (conformist#92)
  # so the linter now FAILS a flake-bearing repo with no version.env instead
  # of silently skipping; under the fleet policy ("adopt version.env where a
  # version exists"), langlang is the versionless case and opts out
  # explicitly. Tracked at langlang#28 — re-enable by adopting version.env if
  # a release process ever lands.
  linters.eng-versioning.enable = lib.mkForce false;

  # langlang is a fork of clarete/langlang (see NOTICE); the fork's own churn
  # is confined to go/ (the tomlcst/extract/junction/binary subsystems) plus
  # this nix layer. rust/ and js/ are untouched since the fork's inception
  # (rust/ has had exactly one commit ever, a directory move by the upstream
  # author) — no Rust formatter is enabled here, but js/ DOES carry Go source
  # (js/wasm/lib/wasm.go, upstream's WASM binding) that the goimports/gofumpt
  # chain would otherwise reach into and reformat, so js/ and rust/ are both
  # excluded explicitly. testdata/ and benchmarks/ similarly carry *.go files
  # the chain would match: testdata/go/*.go are parser test fixtures whose
  # literal byte content is the thing under test, and benchmarks/ is a
  # separate, wholly upstream-authored go.mod module (a third-party-parser
  # comparison harness) this fork's Nix build never touches.
  #
  # grammar_parser_bootstrap.go and tests/binary/wire.go are checked-in
  # `go:generate` OUTPUTS (the bootstrap parser; the sku_record wire codec) —
  # `go generate` regenerates them in the codegen's own style on every build,
  # so reformatting them is pure churn a contributor's next `just build`
  # silently undoes.
  settings.excludes = [
    "flake.lock"
    "go.sum"
    "gomod2nix.toml"
    "*.md"
    "testdata/**"
    "benchmarks/**"
    "js/**"
    "rust/**"
    "go/grammar_parser_bootstrap.go"
    "go/tests/binary/wire.go"
  ];
}
