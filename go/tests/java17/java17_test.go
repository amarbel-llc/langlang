package java17

import (
	"testing"

	langlang "github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const grammarPath = "../../../grammars/java17.peg"
const testdataDir = "../../../testdata/java17"
const java8TestdataDir = "../../../testdata/java8"
const java7TestdataDir = "../../../testdata/java"

// TestJava17TestFiles parses every .java file in the Java 17 testdata directory.
func TestJava17TestFiles(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, testdataDir, ".java")
}

// TestJava17BackwardCompatJava8 verifies that all Java 8 test files still parse.
func TestJava17BackwardCompatJava8(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, java8TestdataDir, ".java")
}

// TestJava17BackwardCompatJava7 verifies that all Java 7 test files still parse.
func TestJava17BackwardCompatJava7(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, java7TestdataDir, ".java")
}

// TestJava17Snippets tests individual Java 17 snippets inline.
func TestJava17Snippets(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	tests := []struct {
		name  string
		input string
	}{
		{"var local", "class A { void m() { var x = 1; } }\n"},
		{"text block", "class A { String s = \"\"\"\n    hello\n    \"\"\"; }\n"},
		{"simple record", "record Point(int x, int y) {}\n"},
		{"record with method", "record P(String n) { public String hello() { return n; } }\n"},
		{"sealed class", "sealed class Shape permits Circle {}\n"},
		{"sealed interface", "sealed interface Expr permits Num {}\n"},
		{"switch arrow", "class A { void m(int x) { switch (x) { case 1 -> {} default -> {} } } }\n"},
		{"pattern instanceof", "class A { void m(Object o) { if (o instanceof String s) {} } }\n"},
		{"yield", "class A { void m(int x) { switch (x) { case 1: yield 10; default: yield 0; } } }\n"},
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

