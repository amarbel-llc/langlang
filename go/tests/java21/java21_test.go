package java21

import (
	"testing"

	langlang "github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const grammarPath = "../../../grammars/java21.peg"
const testdataDir = "../../../testdata/java21"
const java17TestdataDir = "../../../testdata/java17"
const java8TestdataDir = "../../../testdata/java8"
const java7TestdataDir = "../../../testdata/java"

func TestJava21TestFiles(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, testdataDir, ".java")
}

func TestJava21BackwardCompatJava17(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, java17TestdataDir, ".java")
}

func TestJava21BackwardCompatJava8(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, java8TestdataDir, ".java")
}

func TestJava21BackwardCompatJava7(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, java7TestdataDir, ".java")
}

func TestJava21Snippets(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	tests := []struct {
		name  string
		input string
	}{
		// Inherited from Java 17
		{"var local", "class A { void m() { var x = 1; } }\n"},
		{"text block", "class A { String s = \"\"\"\n    hello\n    \"\"\"; }\n"},
		{"simple record", "record Point(int x, int y) {}\n"},
		{"sealed class", "sealed class Shape permits Circle {}\n"},
		{"switch arrow", "class A { void m(int x) { switch (x) { case 1 -> {} default -> {} } } }\n"},
		{"pattern instanceof", "class A { void m(Object o) { if (o instanceof String s) {} } }\n"},
		{"yield", "class A { void m(int x) { switch (x) { case 1: yield 10; default: yield 0; } } }\n"},

		// Java 21: lambda in ternary false branch
		{"lambda in ternary false branch",
			"class A { java.util.function.Predicate<Object> p = true ? x -> true : x -> false; }\n"},
		{"multi-param lambda in ternary",
			"class A { void m(boolean b) { var f = b ? (a, c) -> a : (a, c) -> c; } }\n"},

		// Java 21: type-use annotations in qualified types
		{"annotation in qualified type",
			"class A { Outer.@NotNull Inner field; }\n"},

		// Java 21: instanceof final
		{"instanceof final",
			"class A { void m(Object o) { if (o instanceof final String s) {} } }\n"},

		// Java 21: switch with qualified enum constants
		{"switch qualified enum",
			"class A { void m(E e) { switch (e) { case E.X -> {} case E.Y -> {} default -> {} } } }\n"},
		{"switch string concat case",
			"class A { void m(String s) { switch (s) { case \"a\" + \"b\" -> {} default -> {} } } }\n"},

		// Java 21: local interface in block
		{"local interface",
			"class A { void m() { interface Checker { void check(); } } }\n"},

		// Java 21: record with varargs
		{"record varargs",
			"record R(String... values) {}\n"},

		// Java 21: deconstruction patterns
		{"deconstruction pattern switch",
			"class A { void m(Object o) { switch (o) { case Pair(String a, String b) -> {} default -> {} } } }\n"},
		{"nested deconstruction pattern",
			"class A { void m(Object o) { switch (o) { case Outer(Inner(int x)) -> {} default -> {} } } }\n"},

		// Java 21: qualified super method reference
		{"qualified super method ref",
			"class A implements I { void m() { Runnable r = I.super::method; } }\n"},

		// Java 21: text block with escape
		{"text block escape",
			"class A { String s = \"\"\"\n    line\\\"end\n    \"\"\"; }\n"},
		{"text block with escaped closing",
			"class A { String s = \"\"\"\n    content\\\"\"\"\"; }\n"},
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
