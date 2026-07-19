default: lint build test

lint: lint-fmt lint-worktree

# Read-only formatting + eng-convention gate: builds the `checks.formatting`
# derivation, which runs conformist (nixfmt/goimports/gofumpt + the eng
# linters) against a /nix/store snapshot of the tree and fails if anything
# would change. Does NOT modify the worktree --- that's `codemod-fmt-tree`.
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
[group("pre-build")]
lint-worktree:
    #!/usr/bin/env bash
    set -euo pipefail
    cfg=$(nix build --no-link --print-out-paths '.#conformist-impure-config')
    nix run '.#conformist' -- check --config-file "$cfg" --tree-root .

build: build-go build-generate

# Compile the langlang CLI via the host's go toolchain (fast iteration).
[group("build")]
build-go:
    cd go && go build -o ../build/langlang ./cmd/langlang

# Run `go generate` over go/, using the freshly built langlang binary
# (build-go) on PATH as the codegen tool.
[group("build")]
build-generate: build-go
    cd go && PATH="{{ justfile_directory() }}/build:$PATH" go generate ./...

test: test-go

# Run the Go test suite. Depends on build-generate so generated code exists
# before the tests that exercise it.
[group("post-build")]
test-go: build-generate
    cd go && go test -v ./...

codemod-fmt: codemod-fmt-tree

# Format the tree in place (repair mode) via `nix fmt`.
[group("codemod")]
codemod-fmt-tree:
    nix fmt

clean: clean-go

# Remove the go build cache.
[group("clean")]
clean-go-cache:
    go clean -cache

# Remove the go module download cache.
[group("clean")]
clean-go-modcache:
    go clean -modcache

clean-go: clean-go-cache clean-go-modcache
