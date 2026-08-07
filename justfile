default: lint build test

lint: lint-fmt lint-worktree

# Read-only formatting + eng-convention gate: builds the `checks.formatting`
# derivation, which runs conformist (nixfmt/goimports/gofumpt + the eng
# linters) against a /nix/store snapshot of the tree and fails if anything
# would change. Does NOT modify the worktree --- that's `codemod-fmt-tree`.
#
# check formatting and eng conventions without touching the worktree
[group("pre-build")]
lint-fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    system=$(nix eval --raw --impure --expr 'builtins.currentSystem')
    nix build --no-link --print-build-logs ".#checks.${system}.formatting"

# The impure eng checks (git-remotes, git-default-branch, sweatfile,
# agents-md, gomod2nix) against the working tree, where .git and a real
# go.mod resolution are available --- they can't run in the sandboxed
# checks.formatting. See conformistImpureEval in flake.nix.
#
# run the impure eng checks against the working tree
[group("pre-build")]
lint-worktree:
    #!/usr/bin/env bash
    set -euo pipefail
    cfg=$(nix build --no-link --print-out-paths '.#conformist-impure-config')
    nix run '.#conformist' -- check --config-file "$cfg" --tree-root .

build: build-go build-generate

# compile the langlang CLI via the host's go toolchain (fast iteration)
[group("build")]
build-go:
    cd go && go build -o ../build/langlang ./cmd/langlang

# Run `go generate` over go/, using the freshly built langlang binary
# (build-go) on PATH as the codegen tool.
#
# run `go generate` over go/ with the built langlang binary on PATH
[group("build")]
build-generate: build-go
    cd go && PATH="{{ justfile_directory() }}/build:$PATH" go generate ./...

test: test-go

# Run the Go test suite. Depends on build-generate so generated code exists
# before the tests that exercise it.
#
# run the Go test suite
[group("post-build")]
test-go: build-generate
    cd go && go test -v ./...

codemod-fmt: codemod-fmt-tree

# format the tree in place (repair mode) via `nix fmt`
[group("codemod")]
codemod-fmt-tree:
    nix fmt

# Run the generative grammar fuzzers with a fixed seed and a small case count,
# for quick iteration: the round-trip property (parse(g.String()) == g) and the
# compile-and-run robustness property. See go/roundtrip_gen_test.go and
# go/compile_run_fuzz_test.go.
fuzz seed="1" cases="50":
  cd go && LANGLANG_FUZZ_SEED={{seed}} LANGLANG_FUZZ_CASES={{cases}} go test -v -run Fuzz .

# Sweep the fuzzers across n random seeds (300 cases each) to widen coverage
# beyond the deterministic CI seed.
fuzz-sweep n="12":
  cd go && for s in $(seq 1 {{n}}); do \
    echo "seed $s"; \
    LANGLANG_FUZZ_SEED=$s LANGLANG_FUZZ_CASES=300 go test -run Fuzz . || exit 1; \
  done

clean: clean-go

# remove the go build cache
[group("clean")]
clean-go-cache:
    go clean -cache

# remove the go module download cache
[group("clean")]
clean-go-modcache:
    go clean -modcache

clean-go: clean-go-cache clean-go-modcache
