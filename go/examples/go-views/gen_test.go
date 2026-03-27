package goviews

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/clarete/langlang/go/extract"
)

func grammarPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// go/examples/go-views/ -> go/ -> repo root
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..",
		"grammars", "go.peg")
}

func TestGenerateViews(t *testing.T) {
	if os.Getenv("LANGLANG_GENERATE") == "" {
		t.Skip("set LANGLANG_GENERATE=1 to regenerate views")
	}

	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)

	err := extract.GenerateViews(grammarPath(), "goviews", dir, "SourceFile")
	if err != nil {
		t.Fatal(err)
	}
}
