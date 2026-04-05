package junction

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	goviews "github.com/clarete/langlang/go/examples/go-views"
	jsonviews "github.com/clarete/langlang/go/examples/json-views"
	tomlextract "github.com/clarete/langlang/go/examples/toml-extract"
)

var inputNames = []string{"30kb", "500kb", "2000kb"}

func benchInputs(b *testing.B) map[string][]byte {
	b.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(thisFile), "..", "..",
		"testdata", "json")

	inputs := make(map[string][]byte, len(inputNames))
	for _, name := range inputNames {
		data, err := os.ReadFile(filepath.Join(base, "input_"+name+".json"))
		if err != nil {
			b.Fatalf("read %s: %v", name, err)
		}
		inputs[name] = data
	}
	return inputs
}

// BenchmarkScanOnly measures the junction scanner alone (no partition building).
func BenchmarkScanOnly(b *testing.B) {
	inputs := benchInputs(b)

	for _, name := range inputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			for n := 0; n < b.N; n++ {
				ScanJunctions(input, jsonSpec)
			}
		})
	}
}

// BenchmarkScanAndPartition measures junction scanning + partition tree building.
func BenchmarkScanAndPartition(b *testing.B) {
	inputs := benchInputs(b)

	for _, name := range inputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, jsonSpec)
				BuildPartitions(hits, int32(len(input)))
			}
		})
	}
}

func buildNestedJSON(depth int) []byte {
	var buf []byte
	for range depth {
		buf = append(buf, `{"k": `...)
	}
	buf = append(buf, `[1, true, null, "x"]`...)
	for range depth {
		buf = append(buf, '}')
	}
	return buf
}

// BenchmarkFullParse measures full PEG parsing as the baseline.
func BenchmarkFullParse(b *testing.B) {
	inputs := benchInputs(b)

	for _, name := range inputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			parser := jsonviews.NewJSONParser()
			parser.SetShowFails(false)

			for n := 0; n < b.N; n++ {
				parser.SetInput(input)
				_, err := parser.ParseJSON()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkPartitionParse measures scan + partition + sequential PEG parse
// of each top-level partition.
func BenchmarkPartitionParse(b *testing.B) {
	inputs := benchInputs(b)

	for _, name := range inputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			parser := jsonviews.NewJSONParser()
			parser.SetShowFails(false)

			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, jsonSpec)
				root := BuildPartitions(hits, int32(len(input)))

				for _, part := range root.Children {
					slice := input[part.Start:part.End]
					parser.SetInput(slice)
					_, err := parser.ParseValue()
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

// BenchmarkParallelPartitionParse measures scan + partition + parallel PEG parse
// of the depth-1 child partitions (nested containers within the root).
func BenchmarkParallelPartitionParse(b *testing.B) {
	inputs := benchInputs(b)

	for _, name := range inputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			pool := &sync.Pool{
				New: func() any {
					p := jsonviews.NewJSONParser()
					p.SetShowFails(false)
					return p
				},
			}

			parseFn := func(slice []byte) (any, error) {
				p := pool.Get().(*jsonviews.JSONParser)
				defer pool.Put(p)
				p.SetInput(slice)
				return p.ParseValue()
			}

			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, jsonSpec)
				root := BuildPartitions(hits, int32(len(input)))

				// The test data is a single root array/object. Parallelize
				// across its child partitions (depth-1 containers).
				var parts []Partition
				if len(root.Children) == 1 {
					parts = root.Children[0].Children
				} else {
					parts = root.Children
				}
				ParsePartitions(input, parts, parseFn)
			}
		})
	}
}

// TOML benchmarks

var tomlInputNames = []string{"30kb", "500kb"}

var tomlSpec ScannerSpec

func init() {
	var err error
	tomlSpec, err = AnalyzeForJunctions(tomlGrammarPath())
	if err != nil {
		panic("AnalyzeForJunctions(toml): " + err.Error())
	}
}

func tomlBenchInputs(b *testing.B) map[string][]byte {
	b.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(thisFile), "..", "..",
		"testdata", "toml")

	inputs := make(map[string][]byte, len(tomlInputNames))
	for _, name := range tomlInputNames {
		data, err := os.ReadFile(filepath.Join(base, "input_"+name+".toml"))
		if err != nil {
			b.Fatalf("read %s: %v", name, err)
		}
		inputs[name] = data
	}
	return inputs
}

func BenchmarkTOMLScanOnly(b *testing.B) {
	inputs := tomlBenchInputs(b)

	for _, name := range tomlInputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			for n := 0; n < b.N; n++ {
				ScanJunctions(input, tomlSpec)
			}
		})
	}
}

func BenchmarkTOMLScanAndPartition(b *testing.B) {
	inputs := tomlBenchInputs(b)

	for _, name := range tomlInputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, tomlSpec)
				BuildPartitions(hits, int32(len(input)))
			}
		})
	}
}

func BenchmarkTOMLFullParse(b *testing.B) {
	inputs := tomlBenchInputs(b)

	for _, name := range tomlInputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			parser := tomlextract.NewTOMLParser()
			parser.SetShowFails(false)

			for n := 0; n < b.N; n++ {
				parser.SetInput(input)
				_, err := parser.ParseTOML()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkTOMLParallelPartitionParse(b *testing.B) {
	inputs := tomlBenchInputs(b)

	for _, name := range tomlInputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			pool := &sync.Pool{
				New: func() any {
					p := tomlextract.NewTOMLParser()
					p.SetShowFails(false)
					return p
				},
			}

			parseFn := func(slice []byte) (any, error) {
				p := pool.Get().(*tomlextract.TOMLParser)
				defer pool.Put(p)
				p.SetInput(slice)
				return p.ParseVal()
			}

			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, tomlSpec)
				root := BuildPartitions(hits, int32(len(input)))

				// TOML partitions are arrays and inline tables.
				// Collect all partitions at any depth.
				var parts []Partition
				var collect func(p Partition)
				collect = func(p Partition) {
					for _, c := range p.Children {
						parts = append(parts, c)
						collect(c)
					}
				}
				collect(root)
				ParsePartitions(input, parts, parseFn)
			}
		})
	}
}

// Go benchmarks

var goInputNames = []string{"30kb", "500kb"}

var goSpec ScannerSpec

func init() {
	var err error
	goSpec, err = AnalyzeForJunctions(goGrammarPath())
	if err != nil {
		panic("AnalyzeForJunctions(go): " + err.Error())
	}
}

func goBenchInputs(b *testing.B) map[string][]byte {
	b.Helper()
	goroot := runtime.GOROOT()
	if goroot == "" {
		b.Skip("GOROOT not available")
	}

	files := map[string]string{
		"30kb":  filepath.Join(goroot, "src", "encoding", "json", "decode.go"),
		"500kb": filepath.Join(goroot, "src", "net", "http", "h2_bundle.go"),
	}

	inputs := make(map[string][]byte, len(goInputNames))
	for _, name := range goInputNames {
		data, err := os.ReadFile(files[name])
		if err != nil {
			b.Skipf("skip %s: %v", name, err)
		}
		inputs[name] = data
	}
	return inputs
}

func BenchmarkGoScanOnly(b *testing.B) {
	inputs := goBenchInputs(b)

	for _, name := range goInputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			for n := 0; n < b.N; n++ {
				ScanJunctions(input, goSpec)
			}
		})
	}
}

func BenchmarkGoScanAndPartition(b *testing.B) {
	inputs := goBenchInputs(b)

	for _, name := range goInputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, goSpec)
				BuildPartitions(hits, int32(len(input)))
			}
		})
	}
}

func BenchmarkGoFullParse(b *testing.B) {
	inputs := goBenchInputs(b)

	for _, name := range goInputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			parser := goviews.NewGoParser()
			parser.SetShowFails(false)

			for n := 0; n < b.N; n++ {
				parser.SetInput(input)
				_, err := parser.ParseSourceFile()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGoParallelPartitionParse(b *testing.B) {
	inputs := goBenchInputs(b)

	for _, name := range goInputNames {
		b.Run(name, func(b *testing.B) {
			input := inputs[name]
			b.SetBytes(int64(len(input)))

			pool := &sync.Pool{
				New: func() any {
					p := goviews.NewGoParser()
					p.SetShowFails(false)
					return p
				},
			}

			// Go partitions are blocks ({...}), parameter lists ((...)),
			// etc. ParseBlock() handles the most common container type.
			parseFn := func(slice []byte) (any, error) {
				p := pool.Get().(*goviews.GoParser)
				defer pool.Put(p)
				p.SetInput(slice)
				return p.ParseBlock()
			}

			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, goSpec)
				root := BuildPartitions(hits, int32(len(input)))

				// Collect block partitions ({...}) with valid bounds.
				var parts []Partition
				var collect func(p Partition)
				collect = func(p Partition) {
					for _, c := range p.Children {
						if c.Start >= 0 && c.End > c.Start &&
							c.Start < int32(len(input)) && c.End <= int32(len(input)) &&
							input[c.Start] == '{' {
							parts = append(parts, c)
						}
						collect(c)
					}
				}
				collect(root)
				if len(parts) > 0 {
					ParsePartitions(input, parts, parseFn)
				}
			}
		})
	}
}

// XML benchmarks (scan-only — no PEG parser for XML in this repo)

var xmlSpec = ScannerSpec{
	Junctions: []JunctionByte{
		{'<', JunctionOpen},
		{'>', JunctionClose},
	},
	Sequences: []JunctionSequence{
		{Pattern: []byte("</"), Kind: JunctionClose},
		{Pattern: []byte("/>"), Kind: JunctionClose},
	},
	Quoting: []QuotingContext{
		{Delimiter: '"', EscapePrefix: 0},
	},
}

func buildXML(approxBytes int) []byte {
	// Generate <root><item id="N">value N ...</item>...</root>
	var buf []byte
	buf = append(buf, `<?xml version="1.0"?>`...)
	buf = append(buf, '\n')
	buf = append(buf, `<root>`...)
	buf = append(buf, '\n')
	for i := 0; len(buf) < approxBytes-30; i++ {
		line := fmt.Sprintf(`  <item id="%d">value %d padding data here</item>`, i, i)
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	buf = append(buf, `</root>`...)
	buf = append(buf, '\n')
	return buf
}

func BenchmarkXMLScanOnly(b *testing.B) {
	for _, size := range []struct {
		name  string
		bytes int
	}{
		{"30kb", 30_000},
		{"500kb", 500_000},
	} {
		input := buildXML(size.bytes)
		b.Run(size.name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			for n := 0; n < b.N; n++ {
				ScanJunctions(input, xmlSpec)
			}
		})
	}
}

func BenchmarkXMLScanAndPartition(b *testing.B) {
	for _, size := range []struct {
		name  string
		bytes int
	}{
		{"30kb", 30_000},
		{"500kb", 500_000},
	} {
		input := buildXML(size.bytes)
		b.Run(size.name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, xmlSpec)
				BuildPartitions(hits, int32(len(input)))
			}
		})
	}
}

// BenchmarkNestedScanAndPartition measures scanning + partitioning
// on deeply nested JSON documents.
func BenchmarkNestedScanAndPartition(b *testing.B) {
	for _, depth := range []int{10, 50, 200} {
		input := buildNestedJSON(depth)
		b.Run(fmt.Sprintf("depth%d", depth), func(b *testing.B) {
			b.SetBytes(int64(len(input)))

			for n := 0; n < b.N; n++ {
				hits := ScanJunctions(input, jsonSpec)
				BuildPartitions(hits, int32(len(input)))
			}
		})
	}
}
