package typescript

import (
	"testing"
	"time"

	"github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"

	"github.com/stretchr/testify/require"
)

var corpusSkipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	"vendor":       true,
}

// TestTypeScriptDifferentialCorpus walks a real-world TypeScript
// codebase and parses every .ts file with the TypeScript
// grammar. Since the corpus comes from a working project, every file
// is assumed to be valid TypeScript; any parse failure is a grammar
// gap.
//
//	go test -run TestTypeScriptDifferentialCorpus \
//		-v -timeout 30m ./tests/typescript/   \
//		-args -corpus=~/path/to/ts/repo
func TestTypeScriptDifferentialCorpus(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunFromFlag(t, corpus.Config{
		Extensions:     []string{".ts"},
		Matcher:        matcher,
		SkipDirs:       corpusSkipDirs,
		FailThreshold:  90.0,
		PerFileTimeout: 10 * time.Second,
		LangName:       "TypeScript",
	})
}
