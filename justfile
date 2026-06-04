default: build test

build: build-go

build-go:
  cd go && go build -o ../build/langlang ./cmd/langlang

generate: build-go
  cd go && PATH="{{justfile_directory()}}/build:$PATH" go generate ./...

test: test-go

test-go: generate
  cd go && go test -v ./...

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

clean-go-cache:
  go clean -cache

clean-go-modcache:
  go clean -modcache

clean-go: clean-go-cache clean-go-modcache
