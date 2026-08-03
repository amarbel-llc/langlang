# Nix side of go.mod for langlang's go/ module — the producer half of the
# flake-input-go_mod protocol (amarbel-llc/nixpkgs RFC 0001):
#
#   - producer: mkGoPkgs publishes go-pkgs / go-pkgs-test so downstream
#     repos can bridge code.linenisgreat.com/langlang/go as a flake input
#     (routing that go.mod `require` onto langlang's own go-pkgs output)
#     instead of pinning a go.mod pseudo-version. Motivated by madder's
#     grammar-vectors gate (madder FDR-0010) and hyphence#13.
#
# langlang is polyglot (a Rust workspace under rust/ and a JS/WASM
# playground under js/ alongside this go/ module), so the caller scopes
# src to go/ and downstream consumers bridge with NO subPath — the
# normative single-module producer shape (RFC 0001 § Producer src
# scoping; mirrors piggy's and tommy's go/-scoped producers).
#
# PURE producer: langlang's go/ require graph is entirely public
# (testify, jennifer, ... — no amarbel-llc modules), so there is no
# consumer-side goFlakeInputs bridge to declare (mkGoPkgs.7 § gomod.nix
# CONVENTION, pure-producer shape).
{
  pkgs,
  src,
}:
pkgs.mkGoPkgs {
  inherit src;
  # Explicit name per RFC 0001 Appendix A (amarbel-llc/nixpkgs#49): the
  # module path (code.linenisgreat.com/langlang/go) ends in /go, so the
  # go.mod inference would yield the unhelpful store-path prefix "go"
  # (the same last-segment mismatch conformist.nix documents for the
  # goimports workingDir). Pin it to "langlang" for a repo-prefixed path.
  name = "langlang";
  # builtins.peg (go/ root) is //go:embed'd by grammar_import_loaders.go
  # (const BuiltinsPath = "langlang:builtins.peg") into the root package,
  # which downstream consumers import. mkGoPkgs's default keep-set is
  # *.go + module files, so without this the embed fails in any nix-built
  # consumer with "pattern builtins.peg: no matching files found"
  # (mkGoPkgs.7 § //go:embed ASSETS). Anchored to the single root asset:
  # a blanket ".*\.peg" would also sweep the tests/ and examples/ grammar
  # FIXTURES (12 other *.peg files) into the PROD go-pkgs output.
  extras = [ "builtins\\.peg" ];
}
