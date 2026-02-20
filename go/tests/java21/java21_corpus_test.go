package java21

import (
	"testing"

	langlang "github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"
	"github.com/stretchr/testify/require"
)

var corpusSkipDirs = map[string]bool{
	"build":        true,
	".gradle":      true,
	"node_modules": true,
	".git":         true,
}

// TestJava21DifferentialCorpus walks a real-world Java codebase and
// parses every .java file with the Java 21 grammar. Since the corpus
// comes from a working project, every file is assumed to be valid Java;
// any parse failure is a grammar gap.
//
//	go test -run TestJava21DifferentialCorpus -v -timeout 30m ./tests/java21/ -args -corpus=~/path/to/java/repo
func TestJava21DifferentialCorpus(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunFromFlag(t, corpus.Config{
		Extensions:    []string{".java"},
		Matcher:       matcher,
		SkipDirs:      corpusSkipDirs,
		FailThreshold: 99.0,
		LangName:      "Java 21",
	})
}
