package javascript

import (
	"flag"
	"testing"
	"time"

	"github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"

	"github.com/stretchr/testify/require"
)

// -grammar=es5|esnext selects the grammar (default esnext).
var corpusGrammarFlag = flag.String("grammar", "esnext", "grammar for corpus: es5 or esnext")

var corpusSkipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	".next":        true,
	".nuxt":        true,
	"__pycache__":  true,
	".cache":       true,
	"vendor":       true,
}

// TestJavaScriptDifferentialCorpus walks a real-world JavaScript
// codebase and parses every .js and .mjs file with the selected
// grammar. Since the corpus comes from a working project, every file
// is assumed to be valid JS; any parse failure is a grammar gap.
//
//	go test -run TestJavaScriptDifferentialCorpus -v -timeout 30m \
//		./tests/javascript/ -args -corpus=~/path/to/js/repo -grammar=esnext
func TestJavaScriptDifferentialCorpus(t *testing.T) {
	flag.Parse()
	grammarPath := grammarPathForCorpusFlag(*corpusGrammarFlag)
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunFromFlag(t, corpus.Config{
		Extensions:     []string{".js", ".mjs"},
		Matcher:        matcher,
		SkipDirs:       corpusSkipDirs,
		FailThreshold:  90.0,
		PerFileTimeout: 10 * time.Second,
		LangName:       "JavaScript",
	})
}

func grammarPathForCorpusFlag(name string) string {
	switch name {
	case "es5":
		return grammarES5Path
	case "esnext":
		return grammarESNextPath
	default:
		return grammarESNextPath
	}
}
