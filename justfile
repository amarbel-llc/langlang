default: build test

build: build-go

build-go:
  cd go && go build -o ../build/langlang ./cmd/langlang

generate: build-go
  cd go && PATH="{{justfile_directory()}}/build:$PATH" go generate ./...

test: test-go

test-go: generate
  cd go && go test -v ./...

clean: clean-go

clean-go-cache:
  go clean -cache

clean-go-modcache:
  go clean -modcache

clean-go: clean-go-cache clean-go-modcache
