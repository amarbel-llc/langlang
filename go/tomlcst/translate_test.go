package tomlcst

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	langlang "github.com/clarete/langlang/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func grammarPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "grammars", "toml.peg")
}

func testdataPath(name string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "toml", name)
}

func parseTOML(t *testing.T, input []byte) langlang.Tree {
	t.Helper()
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	cfg.SetBool("compiler.inline.enabled", false)
	db := langlang.NewDatabase(cfg, langlang.NewRelativeImportLoader())
	matcher, err := langlang.QueryMatcher(db, grammarPath())
	require.NoError(t, err, "build matcher")
	tree, _, err := matcher.Match(input)
	require.NoError(t, err, "parse")
	return tree.Copy()
}

// TestTranslateRoundTrip verifies byte-for-byte preservation through
// parse → translate → Bytes().
func TestTranslateRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple kv", "key = \"value\"\n"},
		{"integer", "x = 42\n"},
		{"float", "pi = 3.14\n"},
		{"boolean", "flag = true\n"},
		{"comment only", "# just a comment\n"},
		{"inline comment", "key = \"value\" # comment\n"},
		{"blank lines", "\n\nkey = 1\n\n"},
		{"table", "[server]\nhost = \"localhost\"\nport = 8080\n"},
		{"table with comment", "# config\n[server]\nhost = \"localhost\"  # inline\nport = 8080\n"},
		{"array of tables", "[[products]]\nname = \"Hammer\"\n\n[[products]]\nname = \"Nail\"\n"},
		{"dotted key", "fruit.name = \"apple\"\n"},
		{"array", "ports = [8001, 8002, 8003]\n"},
		{"multiline array", "colors = [\n  \"red\",\n  \"green\",\n  \"blue\",\n]\n"},
		{"inline table", "point = {x = 1, y = 2}\n"},
		{"multiline basic string", "desc = \"\"\"\nHello\nWorld\"\"\"\n"},
		{"literal string", "path = 'C:\\Users\\foo'\n"},
		{"hex integer", "x = 0xDEAD\n"},
		{"special float", "x = inf\n"},
		{"datetime", "dt = 1979-05-27T07:32:00Z\n"},
		{"multiple expressions", "a = 1\nb = 2\nc = 3\n"},
		{"nested tables", "[a]\nx = 1\n\n[a.b]\ny = 2\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := parseTOML(t, []byte(tc.input))
			node := Translate(tree, []byte(tc.input))
			got := string(node.Bytes())
			assert.Equal(t, tc.input, got, "round-trip mismatch")
		})
	}
}

// TestTranslateFixtures verifies round-trip on real-world TOML files.
func TestTranslateFixtures(t *testing.T) {
	fixtures := []string{"input_30kb.toml", "input_500kb.toml", "toml_v1_full.toml"}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(testdataPath(name))
			require.NoError(t, err)

			tree := parseTOML(t, data)
			node := Translate(tree, data)
			got := node.Bytes()
			assert.Equal(t, string(data), string(got), "round-trip mismatch for %s", name)
		})
	}
}

// TestTranslateKeyVal verifies the CST structure for a key-value pair.
func TestTranslateKeyVal(t *testing.T) {
	input := []byte("key = \"value\"\n")
	tree := parseTOML(t, input)
	doc := Translate(tree, input)

	require.Equal(t, NodeDocument, doc.Kind)

	// Find the KeyValue node among the document's children.
	var kv *Node
	for _, c := range doc.Children {
		if c.Kind == NodeKeyValue {
			kv = c
			break
		}
	}
	require.NotNil(t, kv, "expected KeyValue node")

	// KeyVal children should include: Key, WS, Equals, WS, String
	kinds := childKinds(kv)
	t.Logf("KeyVal children: %v", kinds)
	for i, c := range kv.Children {
		t.Logf("  child[%d] kind=%d raw=%q children=%d", i, c.Kind, string(c.Raw), len(c.Children))
	}
	assert.Contains(t, kinds, NodeKey)
	assert.Contains(t, kinds, NodeWhitespace)
	assert.Contains(t, kinds, NodeEquals)
	assert.Contains(t, kinds, NodeString)
}

// TestTranslateTable verifies table header structure.
func TestTranslateTable(t *testing.T) {
	input := []byte("[server]\nhost = \"localhost\"\n")
	tree := parseTOML(t, input)
	doc := Translate(tree, input)

	var tbl *Node
	for _, c := range doc.Children {
		if c.Kind == NodeTable {
			tbl = c
			break
		}
	}
	require.NotNil(t, tbl, "expected Table node")

	// Table children should include BracketOpen, Key, BracketClose
	kinds := childKinds(tbl)
	assert.Contains(t, kinds, NodeBracketOpen)
	assert.Contains(t, kinds, NodeKey)
	assert.Contains(t, kinds, NodeBracketClose)
}

// TestTranslateComment verifies comments become NodeComment nodes.
func TestTranslateComment(t *testing.T) {
	input := []byte("# a comment\nkey = 1\n")
	tree := parseTOML(t, input)
	doc := Translate(tree, input)

	var found bool
	for _, c := range doc.Children {
		if c.Kind == NodeComment {
			assert.Equal(t, "# a comment", string(c.Raw))
			found = true
			break
		}
	}
	assert.True(t, found, "expected Comment node in document children")
}

// TestTranslateFlatten verifies that Trivia, LineEnd, Expression, Val
// wrapper nodes are flattened — their children appear as direct siblings.
func TestTranslateFlatten(t *testing.T) {
	input := []byte("key = 42\n")
	tree := parseTOML(t, input)
	doc := Translate(tree, input)

	// Document should NOT contain nodes named Trivia, LineEnd, Expression, Val.
	// Instead, their children (KeyValue, Newline, WS, etc.) should be direct
	// children of the document.
	for _, c := range doc.Children {
		assert.NotEqual(t, NodeKind(-1), c.Kind,
			"sentinel node should not appear in output")
	}

	// Should find a KeyValue as a direct child of Document.
	kinds := childKinds(doc)
	assert.Contains(t, kinds, NodeKeyValue)
}

// TestTranslateDottedKey verifies dotted key structure.
func TestTranslateDottedKey(t *testing.T) {
	input := []byte("a.b = 1\n")
	tree := parseTOML(t, input)
	doc := Translate(tree, input)

	var kv *Node
	for _, c := range doc.Children {
		if c.Kind == NodeKeyValue {
			kv = c
			break
		}
	}
	require.NotNil(t, kv)

	// First child should be Key containing children with Dot separator.
	var key *Node
	for _, c := range kv.Children {
		if c.Kind == NodeKey {
			key = c
			break
		}
	}
	require.NotNil(t, key, "expected Key node")
	kinds := childKinds(key)
	assert.Contains(t, kinds, NodeDot, "expected Dot node in dotted key")
}

// benchMatcher caches the matcher across benchmark iterations.
func benchMatcher(b *testing.B) langlang.Matcher {
	b.Helper()
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	cfg.SetBool("compiler.inline.enabled", false)
	db := langlang.NewDatabase(cfg, langlang.NewRelativeImportLoader())
	matcher, err := langlang.QueryMatcher(db, grammarPath())
	if err != nil {
		b.Fatal(err)
	}
	return matcher
}

func BenchmarkParseAndTranslate(b *testing.B) {
	matcher := benchMatcher(b)
	fixtures := []string{"input_30kb.toml", "input_500kb.toml"}

	for _, name := range fixtures {
		data, err := os.ReadFile(testdataPath(name))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				tree, _, err := matcher.Match(data)
				if err != nil {
					b.Fatal(err)
				}
				node := Translate(tree, data)
				_ = node.Bytes()
			}
		})
	}
}

func BenchmarkParseOnly(b *testing.B) {
	matcher := benchMatcher(b)
	fixtures := []string{"input_30kb.toml", "input_500kb.toml"}

	for _, name := range fixtures {
		data, err := os.ReadFile(testdataPath(name))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				_, _, err := matcher.Match(data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkTranslateOnly(b *testing.B) {
	matcher := benchMatcher(b)
	fixtures := []string{"input_30kb.toml", "input_500kb.toml"}

	for _, name := range fixtures {
		data, err := os.ReadFile(testdataPath(name))
		if err != nil {
			b.Fatal(err)
		}
		tree, _, err := matcher.Match(data)
		if err != nil {
			b.Fatal(err)
		}
		treeCopy := tree.Copy()

		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				node := Translate(treeCopy, data)
				_ = node.Bytes()
			}
		})
	}
}

func childKinds(n *Node) []NodeKind {
	kinds := make([]NodeKind, len(n.Children))
	for i, c := range n.Children {
		kinds[i] = c.Kind
	}
	return kinds
}
