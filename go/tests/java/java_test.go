package java

import (
	"os"
	"path/filepath"
	"testing"

	langlang "github.com/clarete/langlang/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const grammarPath = "../../../grammars/java1.7.peg"
const testdataDir = "../../../testdata/java"

// TestJavaGrammarTestFiles parses every .java file in the testdata directory
// and asserts that parsing succeeds and consumes the entire input.
func TestJavaGrammarTestFiles(t *testing.T) {
	entries, err := os.ReadDir(testdataDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "no test files found in %s", testdataDir)

	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".java" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(testdataDir, entry.Name()))
			require.NoError(t, err)

			tree, n, err := matcher.Match(data)
			if !assert.NoError(t, err, "failed to parse %s", entry.Name()) {
				if perr, ok := err.(langlang.ParsingError); ok {
					loc := tree.Location(perr.End)
					t.Logf("  error at %d:%d: %s", loc.Line, loc.Column, perr.Message)
				}
				return
			}
			assert.Equal(t, len(data), n, "parser did not consume all input for %s", entry.Name())
		})
	}
}

// TestJavaGrammarSnippets tests individual Java snippets inline.
func TestJavaGrammarSnippets(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	tests := []struct {
		name  string
		input string
	}{
		{"empty class", "class A {}\n"},
		{"class with field", "class A { int x; }\n"},
		{"class with method", "class A { void m() {} }\n"},
		{"return string", "class A { String f() { return \"hi\"; } }\n"},
		{"main method", "class A { public static void main(String[] args) {} }\n"},
		{"array field", "class A { int[] a; }\n"},
		{"constructor", "class A { A() {} }\n"},
		{"simple interface", "interface I { void m(); }\n"},
		{"simple enum", "enum E { A, B, C }\n"},
		{"package and import", "package a; import java.util.List; class A {}\n"},
		{"for loop", "class A { void m() { for (int i = 0; i < 10; i++) {} } }\n"},
		{"while loop", "class A { void m() { while (true) { break; } } }\n"},
		{"if else", "class A { void m() { if (true) {} else {} } }\n"},
		{"try catch finally", "class A { void m() { try {} catch (Exception e) {} finally {} } }\n"},
		{"generic class", "class Box<T> { T value; }\n"},
		{"annotation", "@Deprecated class A {}\n"},
		{"new object", "class A { void m() { Object o = new Object(); } }\n"},
		{"cast", "class A { void m(Object o) { String s = (String) o; } }\n"},
		{"instanceof", "class A { void m(Object o) { boolean b = o instanceof String; } }\n"},
		{"ternary", "class A { void m() { int x = true ? 1 : 0; } }\n"},
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

// BenchmarkJavaGrammar benchmarks parsing the realistic test file.
func BenchmarkJavaGrammar(b *testing.B) {
	cfg := langlang.NewConfig()
	cfg.SetBool("vm.show_fails", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
	require.NoError(b, err)

	data, err := os.ReadFile(filepath.Join(testdataDir, "40_realistic.java"))
	require.NoError(b, err)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, _, err := matcher.Match(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

