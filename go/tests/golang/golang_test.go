package golang

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/clarete/langlang/go"

	"github.com/stretchr/testify/require"
)

const (
	grammarPath = "../../../grammars/go.peg"
	corpusRoot  = "../.."
)

var goSnippets = []struct {
	name string
	src  string
}{
	{"package-import-func", "package p\nimport \"fmt\"\nfunc main(){fmt.Println(1)}\n"},
	{"struct-interface", "package p\ntype S struct{N int}\ntype I interface{M()}\n"},
	{"generic-func", "package p\nfunc Sum[T ~int|~int64](xs []T) T { var z T; for _,v := range xs { z += v }; return z }\n"},
	{"select-chan", "package p\nfunc f(ch chan int){ select { case v := <-ch: _ = v; default: } }\n"},
	{"type-switch", "package p\nfunc f(x any){ switch v := x.(type) { case int: _ = v; default: } }\n"},
}

func TestGoGrammarDifferentialSnippets(t *testing.T) {
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, tc := range goSnippets {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.src)
			_, stdErr := parser.ParseFile(fset, tc.name+".go", data, parser.AllErrors|parser.ParseComments)
			if stdErr != nil {
				t.Fatalf("stdlib parser rejected snippet %s: %v", tc.name, stdErr)
			}
			_, n, err := matcher.Match(data)
			if err != nil {
				t.Fatalf("langlang grammar rejected snippet %s: %v", tc.name, err)
			}
			if n != len(data) {
				t.Fatalf("langlang parser did not consume all snippet input %s: consumed %d of %d", tc.name, n, len(data))
			}
		})
	}
}

func TestGoGrammarDifferentialAgainstStdlibFullCorpus(t *testing.T) {
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
	require.NoError(t, err)
	files := collectGoFiles(corpusRoot)
	fset := token.NewFileSet()
	for _, filename := range files {
		rel := strings.TrimPrefix(filename, corpusRoot+"/")
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(filename)
			if err != nil {
				t.Fatalf("failed to read %s: %v", rel, err)
			}

			_, stdErr := parser.ParseFile(fset, filename, data, parser.AllErrors|parser.ParseComments)
			if stdErr != nil {
				t.Fatalf("stdlib parser rejected file %s: %v", rel, stdErr)
			}

			tree, n, err := matcher.Match(data)
			if err != nil {
				t.Errorf("langlang grammar rejected file %s: %v", rel, err)
				if perr, ok := err.(langlang.ParsingError); ok {
					loc := tree.Location(perr.End)
					t.Logf("  error at %d:%d: %s", loc.Line, loc.Column, perr.Message)
				}
				return
			}
			if n != len(data) {
				t.Errorf("langlang parser did not consume all input for %s: consumed %d of %d bytes", rel, n, len(data))
			}
		})
	}
}

var sampledCorpusFiles = []string{
	"../../api.go",
	"../../grammar_import.go",
	"../../grammar_parser_bootstrap.go", // that bytecode table is big
	"../../query_analysis.go",
	"../../tests/arithmetic_leftrec/arithmetic_test.go",
}

// BenchmarkParser benchmarks the langlang matcher on Go snippets and
// real .go files.  This follows the naming convention used by the
// other benchmark suites so the rb script can pick it up.
func BenchmarkParser(b *testing.B) {
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
	if err != nil {
		b.Fatalf("failed to create matcher: %v", err)
	}
	for _, tc := range goSnippets {
		data := []byte(tc.src)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				_, _, err := matcher.Match(data)
				if err != nil {
					b.Fatalf("match error: %v", err)
				}
			}
		})
	}
	for _, filename := range sampledCorpusFiles {
		data, err := os.ReadFile(filename)
		if err != nil {
			b.Fatalf("failed to read %s: %v", filename, err)
		}
		rel := strings.TrimPrefix(filename, corpusRoot+"/")
		b.Run("file/"+rel, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				_, _, err := matcher.Match(data)
				if err != nil {
					b.Fatalf("match error on %s: %v", rel, err)
				}
			}
		})
	}
}

// BenchmarkStdlib benchmarks Go's standard library parser on the same
// inputs, for comparison with BenchmarkParser.
func BenchmarkStdlib(b *testing.B) {
	fset := token.NewFileSet()
	for _, tc := range goSnippets {
		data := []byte(tc.src)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				_, err := parser.ParseFile(fset, tc.name+".go", data, parser.AllErrors|parser.ParseComments)
				if err != nil {
					b.Fatalf("stdlib parse error: %v", err)
				}
			}
		})
	}
	for _, filename := range sampledCorpusFiles {
		data, err := os.ReadFile(filename)
		if err != nil {
			b.Fatalf("failed to read %s: %v", filename, err)
		}
		rel := strings.TrimPrefix(filename, corpusRoot+"/")
		b.Run("file/"+rel, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				_, err := parser.ParseFile(fset, filename, data, parser.AllErrors|parser.ParseComments)
				if err != nil {
					b.Fatalf("stdlib parse error on %s: %v", rel, err)
				}
			}
		})
	}
}

func collectGoFiles(root string) []string {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk corpus root %s: %v", root, err))
	}
	sort.Strings(files)
	if len(files) == 0 {
		panic(fmt.Errorf("no .go files found under %s", root))
	}
	return files
}
