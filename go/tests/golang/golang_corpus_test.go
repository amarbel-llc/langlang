package golang

import (
	"testing"

	"github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"

	"github.com/stretchr/testify/require"
)

var corpusSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
}

// TestGoDifferentialCorpus walks a real-world Go codebase and parses
// every .go file with the Go grammar. Since the corpus comes from a
// working project, every file is assumed to be valid Go; any parse
// failure is a grammar gap.
//
//	go test -run TestGoDifferentialCorpus -v -timeout 30m \
//		 ./tests/golang/ -args -corpus=~/path/to/go/repo
func TestGoDifferentialCorpus(t *testing.T) {
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
	require.NoError(t, err)
	corpus.RunFromFlag(t, corpus.Config{
		Extensions:    []string{".go"},
		Matcher:       matcher,
		SkipDirs:      corpusSkipDirs,
		FailThreshold: 90.0,
		LangName:      "Go",
	})
}
