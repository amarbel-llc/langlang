package junction

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	jsonviews "github.com/clarete/langlang/go/examples/json-views"
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
	for i := 0; i < depth; i++ {
		buf = append(buf, `{"k": `...)
	}
	buf = append(buf, `[1, true, null, "x"]`...)
	for i := 0; i < depth; i++ {
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
