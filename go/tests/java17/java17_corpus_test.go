package java17

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

// TestJava17DifferentialCorpus walks a real-world Java codebase and
// parses every .java file with the Java 17 grammar. Since the corpus
// comes from a working project, every file is assumed to be valid Java;
// any parse failure is a grammar gap.
//
//	go test -run TestJava17DifferentialCorpus -v -timeout 30m ./tests/java17/ -args -corpus=~/path/to/java/repo
func TestJava17DifferentialCorpus(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunFromFlag(t, corpus.Config{
		Extensions:    []string{".java"},
		Matcher:       matcher,
		SkipDirs:      corpusSkipDirs,
		FailThreshold: 90.0,
		LangName:      "Java 17",
	})
}
