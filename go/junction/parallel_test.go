package junction

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	jsonviews "github.com/clarete/langlang/go/examples/json-views"
)

func TestParallelPartitionParse(t *testing.T) {
	spec, err := AnalyzeForJunctions(jsonGrammarPath())
	if err != nil {
		t.Fatalf("AnalyzeForJunctions: %v", err)
	}

	_, thisFile, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(thisFile), "..", "..",
		"testdata", "json")

	for _, name := range []string{"30kb", "500kb"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(base, "input_"+name+".json"))
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			hits := ScanJunctions(input, spec)
			root := BuildPartitions(hits, int32(len(input)))

			if len(root.Children) != 1 {
				t.Fatalf("expected 1 top-level partition, got %d", len(root.Children))
			}

			parts := root.Children[0].Children
			t.Logf("parallelizing across %d depth-1 partitions", len(parts))

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
				tree, err := p.ParseValue()
				if err != nil {
					return nil, err
				}
				return tree.Copy(), nil
			}

			results := ParsePartitions(input, parts, parseFn)

			for i, r := range results {
				if r.Err != nil {
					t.Errorf("partition[%d] parse error: %v", i, r.Err)
					continue
				}
				tree := r.Value.(jsonviews.Tree)
				root, ok := tree.Root()
				if !ok {
					t.Errorf("partition[%d] no root", i)
					continue
				}
				text := tree.Text(root)
				slice := input[r.Partition.Start:r.Partition.End]
				if text != string(slice) {
					t.Errorf("partition[%d] text mismatch:\n  got:  %q\n  want: %q",
						i, truncate([]byte(text), 80), truncate(slice, 80))
				}
			}
		})
	}
}
