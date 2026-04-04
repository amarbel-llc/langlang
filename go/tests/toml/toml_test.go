package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

// TestParseTOMLv1Full verifies parsing of a comprehensive TOML v1.0
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
