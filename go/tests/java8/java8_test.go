package java8

import (
	"testing"

	"github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	grammarPath      = "../../../grammars/java8.peg"
	testdataDir      = "../../../testdata/java8"
	java7TestdataDir = "../../../testdata/java"
)

// TestJava8TestFiles parses every .java file in the Java 8 testdata directory.
func TestJava8TestFiles(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, testdataDir, ".java")
}

// TestJava8BackwardCompat verifies that all Java 7 test files still parse.
func TestJava8BackwardCompat(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, java7TestdataDir, ".java")
}

// TestJava8Snippets tests individual Java 8 snippets inline.
func TestJava8Snippets(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	tests := []struct {
		name  string
		input string
	}{
		{"lambda no params", "class A { Runnable r = () -> {}; }\n"},
		{"lambda single param", "class A { void m() { foo(x -> x + 1); } }\n"},
		{"lambda multi params", "class A { void m() { compare((a, b) -> a - b); } }\n"},
		{"lambda typed params", "class A { void m() { compare((int a, int b) -> a - b); } }\n"},
		{"lambda block body", "class A { void m() { foo(() -> { return 1; }); } }\n"},
		{"method ref static", "class A { void m() { foo(String::valueOf); } }\n"},
		{"method ref constructor", "class A { void m() { foo(ArrayList::new); } }\n"},
		{"default interface method", "interface A { default void m() {} }\n"},
		{"static interface method", "interface A { static int m() { return 0; } }\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, n, err := matcher.Match([]byte(tt.input))
			if !assert.NoError(t, err, "failed to parse: %s", tt.name) {
				if perr, ok := err.(langlang.ParsingError); ok {
					loc := tree.Location(perr.End)
					t.Logf("  error at %d:%d: %s", loc.Line, loc.Column, perr.Message)
				}
				return
			}
			assert.Equal(t, len(tt.input), n, "parser did not consume all input for: %s", tt.name)
		})
	}
}
