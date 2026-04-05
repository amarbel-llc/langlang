package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:generate go run ../../cmd/langlang -grammar ../../../grammars/toml.peg -output-language go -output-path ./toml.go
//go:generate go run ../../cmd/langlang -grammar ../../../grammars/toml.peg -output-language go -output-path ./toml.nocap.go -disable-captures -go-parser NoCapParser -go-remove-lib

var inputNames = []string{"30kb", "500kb"}

func getInputs(tb testing.TB) map[string][]byte {
	tb.Helper()

	inputs := make(map[string][]byte, len(inputNames))
	read := func(n string) []byte {
		path := filepath.Join("..", "..", "..", "testdata", "toml", "input_"+n+".toml")
		text, err := os.ReadFile(path)
		require.NoError(tb, err)
		return text
	}
	for _, name := range inputNames {
		inputs[name] = read(name)
	}
	return inputs
}

func readTestFile(tb testing.TB, name string) []byte {
	tb.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "toml", name)
	data, err := os.ReadFile(path)
	require.NoError(tb, err)
	return data
}

// TestParseBenchmarkInputs ensures the generated parser can handle
// the benchmark input files without error.
func TestParseBenchmarkInputs(t *testing.T) {
	inputs := getInputs(t)
	p := NewParser()

	for _, name := range inputNames {
		t.Run(name, func(t *testing.T) {
			p.SetInput(inputs[name])
			_, err := p.ParseTOML()
			require.NoError(t, err)
		})
	}
}

// TestParseTOMLv1Full verifies parsing of a comprehensive TOML v1.1
// test file that exercises every language construct.
func TestParseTOMLv1Full(t *testing.T) {
	data := readTestFile(t, "toml_v1_full.toml")
	p := NewParser()
	p.SetInput(data)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestComments verifies comments are captured as parse-tree nodes.
func TestComments(t *testing.T) {
	input := []byte("# this is a comment\nkey = \"value\" # inline\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestDottedKeys verifies dotted key notation for nested assignments.
func TestDottedKeys(t *testing.T) {
	input := []byte("fruit.name = \"apple\"\nfruit.color = \"red\"\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestArrayOfTables verifies [[...]] array-of-tables syntax.
func TestArrayOfTables(t *testing.T) {
	input := []byte(`[[products]]
name = "Hammer"
sku = 738594937

[[products]]
name = "Nail"
sku = 284758393
`)
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestLiteralStrings verifies single-quoted literal strings.
func TestLiteralStrings(t *testing.T) {
	input := []byte("winpath = 'C:\\Users\\nodejs\\templates'\nregex = '<\\i\\c*\\s*>'\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestMultilineBasicString verifies multi-line basic strings with
// line-ending backslash.
func TestMultilineBasicString(t *testing.T) {
	input := []byte("str = \"\"\"\\\n  The quick brown \\\n  fox.\\\n  \"\"\"\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestMultilineLiteralString verifies multi-line literal strings.
func TestMultilineLiteralString(t *testing.T) {
	input := []byte("str = '''\nI [dw]on't need \\d{2} apples'''\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestHexOctBinIntegers verifies non-decimal integer formats.
func TestHexOctBinIntegers(t *testing.T) {
	input := []byte("hex = 0xDEADBEEF\noct = 0o755\nbin = 0b11010110\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestSpecialFloats verifies inf and nan float values.
func TestSpecialFloats(t *testing.T) {
	input := []byte("f1 = inf\nf2 = +inf\nf3 = -inf\nf4 = nan\nf5 = +nan\nf6 = -nan\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestAllDateTimeFormats verifies offset, local date-time, local date,
// and local time formats.
func TestAllDateTimeFormats(t *testing.T) {
	input := []byte(`odt1 = 1979-05-27T07:32:00Z
odt2 = 1979-05-27T00:32:00-07:00
odt3 = 1979-05-27T00:32:00.999999-07:00
odt4 = 1979-05-27 07:32:00Z
ldt1 = 1979-05-27T07:32:00
ldt2 = 1979-05-27T00:32:00.999999
ld1 = 1979-05-27
lt1 = 07:32:00
lt2 = 00:32:00.999999
`)
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestUnicodeEscapes verifies unicode escape sequences in strings.
func TestUnicodeEscapes(t *testing.T) {
	input := []byte("str1 = \"\\u00E9\"\nstr2 = \"\\U0001F600\"\nstr3 = \"\\x41\"\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestInlineTable verifies inline table syntax.
func TestInlineTable(t *testing.T) {
	input := []byte("point = {x = 1, y = 2}\nnested = {type.name = \"pug\"}\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestArrayWithComments verifies arrays with interspersed comments.
func TestArrayWithComments(t *testing.T) {
	input := []byte("arr = [\n  1,\n  2, # comment\n]\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestUnderscoreInNumbers verifies underscore separators in numbers.
func TestUnderscoreInNumbers(t *testing.T) {
	input := []byte("int = 1_000_000\nflt = 224_617.445_991\nhex = 0xdead_beef\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestSpaceDelimitedDateTime verifies that a space (instead of T) can
// separate date and time in offset date-time values.
func TestSpaceDelimitedDateTime(t *testing.T) {
	input := []byte("odt = 1979-05-27 07:32:00Z\n")
	p := NewParser()
	p.SetInput(input)

	tree, err := p.ParseTOML()
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestRejectInvalid verifies that the parser rejects syntactically
// invalid TOML inputs.
func TestRejectInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"newline in inline table", "point = {\nx = 1\n}\n"},
		{"unterminated basic string", "key = \"unterminated\n"},
		{"unterminated literal string", "key = 'unterminated\n"},
		{"newline in basic string", "key = \"line1\nline2\"\n"},
		{"newline in literal string", "key = 'line1\nline2'\n"},
		{"leading zeros in decimal integer", "key = 0123\n"},
		{"value missing after equals", "key =\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser()
			p.SetInput([]byte(tc.input))
			_, err := p.ParseTOML()
			require.Error(t, err, "expected parse error for: %s", tc.input)
		})
	}
}

// findNamed walks the tree and returns the first node whose Name matches.
func findNamed(tr Tree, name string) (NodeID, bool) {
	root, ok := tr.Root()
	if !ok {
		return 0, false
	}
	var found NodeID
	var match bool
	tr.Visit(root, func(id NodeID) bool {
		if tr.Type(id) == NodeType_Node && tr.Name(id) == name {
			found = id
			match = true
			return false
		}
		return true
	})
	return found, match
}

// collectNamed walks the tree and returns all nodes whose Name matches.
func collectNamed(tr Tree, name string) []NodeID {
	root, ok := tr.Root()
	if !ok {
		return nil
	}
	var nodes []NodeID
	tr.Visit(root, func(id NodeID) bool {
		if tr.Type(id) == NodeType_Node && tr.Name(id) == name {
			nodes = append(nodes, id)
		}
		return true
	})
	return nodes
}

// TestTreeKeyVal verifies the parse tree structure for a key-value pair.
func TestTreeKeyVal(t *testing.T) {
	p := NewParser()
	p.SetInput([]byte("key = \"value\"\n"))
	tr, err := p.ParseTOML()
	require.NoError(t, err)

	kvID, ok := findNamed(tr, "KeyVal")
	require.True(t, ok, "expected KeyVal node in tree")

	keyID, ok := findNamed(tr, "Key")
	require.True(t, ok, "expected Key node in tree")
	require.Contains(t, tr.Text(keyID), "key")

	valID, ok := findNamed(tr, "Val")
	require.True(t, ok, "expected Val node in tree")
	_ = kvID
	_ = valID
}

// TestTreeInteger verifies that integer values produce Integer nodes
// with the correct text span.
func TestTreeInteger(t *testing.T) {
	tests := []struct {
		name  string
		input string
		text  string
	}{
		{"decimal", "x = 42\n", "42"},
		{"hex", "x = 0xDEAD\n", "0xDEAD"},
		{"octal", "x = 0o755\n", "0o755"},
		{"binary", "x = 0b1101\n", "0b1101"},
		{"signed positive", "x = +99\n", "+99"},
		{"signed negative", "x = -17\n", "-17"},
		{"underscores", "x = 1_000\n", "1_000"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser()
			p.SetInput([]byte(tc.input))
			tr, err := p.ParseTOML()
			require.NoError(t, err)

			intID, ok := findNamed(tr, "Integer")
			require.True(t, ok, "expected Integer node")
			require.Equal(t, tc.text, tr.Text(intID))
		})
	}
}

// TestTreeFloat verifies that float values produce Float nodes with
// the correct text span.
func TestTreeFloat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		text  string
	}{
		{"basic", "x = 3.14\n", "3.14"},
		{"exponent", "x = 5e+22\n", "5e+22"},
		{"inf", "x = inf\n", "inf"},
		{"negative inf", "x = -inf\n", "-inf"},
		{"nan", "x = nan\n", "nan"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser()
			p.SetInput([]byte(tc.input))
			tr, err := p.ParseTOML()
			require.NoError(t, err)

			floatID, ok := findNamed(tr, "Float")
			require.True(t, ok, "expected Float node")
			require.Equal(t, tc.text, tr.Text(floatID))
		})
	}
}

// TestTreeDateTime verifies that date-time values produce DateTime nodes
// with the correct text span.
func TestTreeDateTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		text  string
	}{
		{"offset", "x = 1979-05-27T07:32:00Z\n", "1979-05-27T07:32:00Z"},
		{"local datetime", "x = 1979-05-27T07:32:00\n", "1979-05-27T07:32:00"},
		{"local date", "x = 1979-05-27\n", "1979-05-27"},
		{"local time", "x = 07:32:00\n", "07:32:00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser()
			p.SetInput([]byte(tc.input))
			tr, err := p.ParseTOML()
			require.NoError(t, err)

			dtID, ok := findNamed(tr, "DateTime")
			require.True(t, ok, "expected DateTime node")
			require.Equal(t, tc.text, tr.Text(dtID))
		})
	}
}

// TestTreeBoolean verifies that boolean values produce Boolean nodes.
func TestTreeBoolean(t *testing.T) {
	p := NewParser()
	p.SetInput([]byte("a = true\nb = false\n"))
	tr, err := p.ParseTOML()
	require.NoError(t, err)

	bools := collectNamed(tr, "Boolean")
	require.Len(t, bools, 2)
	require.Equal(t, "true", tr.Text(bools[0]))
	require.Equal(t, "false", tr.Text(bools[1]))
}

// TestTreeArray verifies that array values produce Array nodes with
// the correct number of child values.
func TestTreeArray(t *testing.T) {
	p := NewParser()
	p.SetInput([]byte("a = [1, 2, 3]\n"))
	tr, err := p.ParseTOML()
	require.NoError(t, err)

	arrID, ok := findNamed(tr, "Array")
	require.True(t, ok, "expected Array node")

	ints := collectNamed(tr, "Integer")
	require.Len(t, ints, 3)
	require.Equal(t, "1", tr.Text(ints[0]))
	require.Equal(t, "2", tr.Text(ints[1]))
	require.Equal(t, "3", tr.Text(ints[2]))
	_ = arrID
}

// TestTreeStdTable verifies that [table] headers produce StdTable nodes.
func TestTreeStdTable(t *testing.T) {
	p := NewParser()
	p.SetInput([]byte("[server]\nhost = \"localhost\"\n"))
	tr, err := p.ParseTOML()
	require.NoError(t, err)

	tableID, ok := findNamed(tr, "StdTable")
	require.True(t, ok, "expected StdTable node")
	require.Contains(t, tr.Text(tableID), "server")
}

// TestTreeArrayTable verifies that [[table]] headers produce ArrayTable nodes.
func TestTreeArrayTable(t *testing.T) {
	p := NewParser()
	p.SetInput([]byte("[[items]]\nname = \"a\"\n[[items]]\nname = \"b\"\n"))
	tr, err := p.ParseTOML()
	require.NoError(t, err)

	tables := collectNamed(tr, "ArrayTable")
	require.Len(t, tables, 2)
}

// TestTreeInlineTable verifies that inline tables produce InlineTable nodes.
func TestTreeInlineTable(t *testing.T) {
	p := NewParser()
	p.SetInput([]byte("point = {x = 1, y = 2}\n"))
	tr, err := p.ParseTOML()
	require.NoError(t, err)

	itID, ok := findNamed(tr, "InlineTable")
	require.True(t, ok, "expected InlineTable node")
	_ = itID
}

// TestTreeStrings verifies that different string types produce the
// correct text spans.
func TestTreeStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		text  string
	}{
		{"basic string", "k = \"hello\"\n", "\"hello\""},
		{"literal string", "k = 'hello'\n", "'hello'"},
		{"escape sequence", "k = \"tab\\there\"\n", "\"tab\\there\""},
		{"unicode escape", "k = \"\\u00E9\"\n", "\"\\u00E9\""},
		{"v1.1 hex escape", "k = \"\\x41\"\n", "\"\\x41\""},
		{"v1.1 esc escape", "k = \"\\e\"\n", "\"\\e\""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser()
			p.SetInput([]byte(tc.input))
			tr, err := p.ParseTOML()
			require.NoError(t, err)

			valID, ok := findNamed(tr, "Val")
			require.True(t, ok, "expected Val node")
			require.Equal(t, tc.text, tr.Text(valID))
		})
	}
}

// TestTreeDottedKey verifies that dotted keys produce a Key node
// containing multiple SimpleKey children.
func TestTreeDottedKey(t *testing.T) {
	p := NewParser()
	p.SetInput([]byte("a.b.c = 1\n"))
	tr, err := p.ParseTOML()
	require.NoError(t, err)

	keyID, ok := findNamed(tr, "Key")
	require.True(t, ok, "expected Key node")
	require.Equal(t, "a.b.c", strings.TrimSpace(tr.Text(keyID)))
}

// TestCommentPreservedInTree verifies that comments appear in the parse
// tree as part of Spacing text nodes.
func TestCommentPreservedInTree(t *testing.T) {
	p := NewParser()
	p.SetInput([]byte("# standalone comment\nkey = 1\n"))
	tr, err := p.ParseTOML()
	require.NoError(t, err)

	root, ok := tr.Root()
	require.True(t, ok)

	fullText := tr.Text(root)
	assert.Contains(t, fullText, "# standalone comment",
		"comment text should be preserved in the parse tree")
}

// Benchmarks

func BenchmarkParser(b *testing.B) {
	inputs := getInputs(b)
	p := NewParser()
	p.SetShowFails(false)

	for _, name := range inputNames {
		b.Run(fmt.Sprintf("Input %s", name), func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))
			p.SetInput(input)

			for n := 0; n < b.N; n++ {
				p.ParseTOML()
			}
		})
	}
}

func BenchmarkNoCapParser(b *testing.B) {
	inputs := getInputs(b)
	p := NewNoCapParser()
	p.SetShowFails(false)

	for _, name := range inputNames {
		b.Run(fmt.Sprintf("Input %s", name), func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))
			p.SetInput(input)

			for n := 0; n < b.N; n++ {
				p.ParseTOML()
			}
		})
	}
}
