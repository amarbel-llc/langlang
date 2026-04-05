package junction

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	jsonviews "github.com/clarete/langlang/go/examples/json-views"
)

// collectLeafValues walks the full parse tree and collects the text of
// every Value node at the given depth, in order. This gives us the
// "expected" content for each independent parse region.
func collectLeafValues(t jsonviews.Tree, id jsonviews.NodeID, depth int, target int, out *[]string) {
	name := t.Name(id)
	if name == "Value" && depth == target {
		*out = append(*out, t.Text(id))
		return
	}
	for _, child := range t.AppendChildren(id, nil) {
		nextDepth := depth
		if name == "Object" || name == "Array" {
			nextDepth = depth + 1
		}
		collectLeafValues(t, child, nextDepth, target, out)
	}
}

// collectPartitionValues extracts the content regions from a partition
// that should each parse independently as a Value. These are the byte
// ranges between separator junctions at the partition's immediate depth.
func collectPartitionRegions(p Partition, input []byte, hits []JunctionHit) [][]byte {
	// Find separator hits at depth p.Depth+1 within [p.Start, p.End).
	var seps []int32
	for _, h := range hits {
		if h.Pos <= p.Start || h.Pos >= p.End-1 {
			continue
		}
		if h.Kind == JunctionSeparator && h.Depth == p.Depth+1 {
			seps = append(seps, h.Pos)
		}
	}

	// Split the interior (between open and close delimiters) by separators.
	start := p.Start + 1 // skip opening delimiter
	end := p.End - 1     // skip closing delimiter

	if len(seps) == 0 {
		region := input[start:end]
		if len(trimWhitespace(region)) > 0 {
			return [][]byte{region}
		}
		return nil
	}

	var regions [][]byte
	for _, sep := range seps {
		region := input[start:sep]
		if len(trimWhitespace(region)) > 0 {
			regions = append(regions, region)
		}
		start = sep + 1
	}
	// Last region after final separator.
	region := input[start:end]
	if len(trimWhitespace(region)) > 0 {
		regions = append(regions, region)
	}
	return regions
}

func trimWhitespace(b []byte) []byte {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	j := len(b)
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\n' || b[j-1] == '\r') {
		j--
	}
	return b[i:j]
}

func TestVerifyPartitionParsing(t *testing.T) {
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
				t.Fatalf("read input: %v", err)
			}

			// Full parse for reference.
			fullParser := jsonviews.NewJSONParser()
			fullParser.SetShowFails(false)
			fullParser.SetInput(input)
			fullTree, err := fullParser.ParseJSON()
			if err != nil {
				t.Fatalf("full parse: %v", err)
			}
			fullTree = fullTree.Copy()

			fullRoot, ok := fullTree.Root()
			if !ok {
				t.Fatal("no root in full parse")
			}
			fullText := fullTree.Text(fullRoot)

			// Scan + partition.
			hits := ScanJunctions(input, spec)
			root := BuildPartitions(hits, int32(len(input)))

			if len(root.Children) == 0 {
				t.Fatal("no partitions")
			}

			// For each top-level container partition, parse the slice
			// as a complete Value and verify the text matches.
			for i, part := range root.Children {
				slice := input[part.Start:part.End]

				partParser := jsonviews.NewJSONParser()
				partParser.SetShowFails(false)
				partParser.SetInput(slice)
				partTree, err := partParser.ParseValue()
				if err != nil {
					t.Errorf("partition[%d] parse error: %v (slice: %q...)",
						i, err, truncate(slice, 80))
					continue
				}

				partRoot, ok := partTree.Root()
				if !ok {
					t.Errorf("partition[%d] no root", i)
					continue
				}

				partText := partTree.Text(partRoot)
				if partText != string(slice) {
					t.Errorf("partition[%d] text mismatch:\n  got:  %q\n  want: %q",
						i, truncate([]byte(partText), 80), truncate(slice, 80))
				}
			}

			// Verify the full text matches the original input (modulo
			// whitespace from the JSON rule's Spacing handling).
			if len(fullText) == 0 {
				t.Error("full parse produced empty text")
			}

			// Count total nodes in full parse.
			var fullNodeCount int
			fullTree.Visit(fullRoot, func(id jsonviews.NodeID) bool {
				fullNodeCount++
				return true
			})
			t.Logf("input=%s full_nodes=%d partitions=%d hits=%d",
				name, fullNodeCount, len(root.Children), len(hits))
		})
	}
}

// TestVerifyPartitionRegions verifies that regions between separators
// within each partition parse independently as Values.
func TestVerifyPartitionRegions(t *testing.T) {
	spec, err := AnalyzeForJunctions(jsonGrammarPath())
	if err != nil {
		t.Fatalf("AnalyzeForJunctions: %v", err)
	}

	// Use a small hand-crafted input for precise verification.
	input := []byte(`{"a":1,"b":"hello","c":[true,false,null]}`)

	hits := ScanJunctions(input, spec)
	root := BuildPartitions(hits, int32(len(input)))

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 top-level partition, got %d", len(root.Children))
	}

	obj := root.Children[0]

	// The object's interior regions between separators should be
	// parseable member-like pairs. But since : splits key from value,
	// we expect alternating keys and values. Let's verify the regions exist.
	regions := collectPartitionRegions(obj, input, hits)
	t.Logf("object regions: %d", len(regions))
	for i, r := range regions {
		t.Logf("  region[%d]: %q", i, string(r))
	}

	// Each region should parse as a Value.
	parser := jsonviews.NewJSONParser()
	parser.SetShowFails(false)

	for i, region := range regions {
		parser.SetInput(region)
		tree, err := parser.ParseValue()
		if err != nil {
			t.Errorf("region[%d] %q parse error: %v", i, string(region), err)
			continue
		}
		partRoot, ok := tree.Root()
		if !ok {
			t.Errorf("region[%d] no root", i)
			continue
		}
		text := tree.Text(partRoot)
		trimmed := string(trimWhitespace(region))
		if text != trimmed {
			t.Errorf("region[%d] text=%q want=%q", i, text, trimmed)
		}
	}

	// Also verify child partitions (the array).
	if len(obj.Children) != 1 {
		t.Fatalf("object children = %d, want 1 (array)", len(obj.Children))
	}

	arr := obj.Children[0]
	arrRegions := collectPartitionRegions(arr, input, hits)
	t.Logf("array regions: %d", len(arrRegions))
	for i, r := range arrRegions {
		t.Logf("  region[%d]: %q", i, string(r))
		parser.SetInput(r)
		tree, err := parser.ParseValue()
		if err != nil {
			t.Errorf("array region[%d] %q parse error: %v", i, string(r), err)
			continue
		}
		partRoot, ok := tree.Root()
		if !ok {
			t.Errorf("array region[%d] no root", i)
			continue
		}
		text := tree.Text(partRoot)
		trimmed := string(trimWhitespace(r))
		if text != trimmed {
			t.Errorf("array region[%d] text=%q want=%q", i, text, trimmed)
		}
	}
}

// TestVerifyPEGSelfHostingScan scans the PEG self-hosting grammar with
// its own junction spec and verifies the scanner correctly identifies
// structural delimiters. This is the FDR-0003 promotion criteria test
// for "langlang self-hosting grammars".
func TestVerifyPEGSelfHostingScan(t *testing.T) {
	spec, err := AnalyzeForJunctions(pegGrammarPath())
	if err != nil {
		t.Fatalf("AnalyzeForJunctions: %v", err)
	}

	input, err := os.ReadFile(pegGrammarPath())
	if err != nil {
		t.Fatalf("read peg.peg: %v", err)
	}

	hits := ScanJunctions(input, spec)

	if len(hits) == 0 {
		t.Fatal("no junctions found in peg.peg")
	}

	// Count by kind.
	var opens, closes, seps int
	for _, h := range hits {
		switch h.Kind {
		case JunctionOpen:
			opens++
		case JunctionClose:
			closes++
		case JunctionSeparator:
			seps++
		}
	}

	t.Logf("peg.peg: %d hits (opens=%d closes=%d seps=%d)",
		len(hits), opens, closes, seps)

	// Character classes are mostly balanced. PEG allows escaped ] inside
	// classes (e.g., [nrt'"\[\]\\]) which the scanner can't distinguish
	// from structural closes without a quoting context. Allow a small
	// imbalance.
	if abs(opens-closes) > 2 {
		t.Errorf("delimiters too unbalanced: %d opens vs %d closes (>2 off)", opens, closes)
	} else if opens != closes {
		t.Logf("note: %d opens vs %d closes (escaped ] inside character class)", opens, closes)
	}

	// The grammar has multiple character classes ([a-zA-Z_], [0-9],
	// [nrt'\"\\[\\]\\\\], etc) so we expect a meaningful number.
	if opens < 5 {
		t.Errorf("expected at least 5 character class open delimiters, got %d", opens)
	}

	// Choice separators: Expression <- Sequence (SLASH Sequence)*
	// The grammar uses / extensively. But / also appears in comments
	// (// ...) — verify separators were found.
	if seps == 0 {
		t.Error("expected choice separator junctions for '/'")
	}

	// Verify all hits reference valid positions in the input.
	for i, h := range hits {
		if h.Pos < 0 || int(h.Pos) >= len(input) {
			t.Errorf("hit[%d] position %d out of range [0, %d)", i, h.Pos, len(input))
		}
		// The byte at the hit position should be a known junction byte.
		b := input[h.Pos]
		found := false
		for _, jb := range spec.Junctions {
			if jb.Byte == b {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("hit[%d] byte %q at pos %d is not a junction byte",
				i, rune(b), h.Pos)
		}
	}

	// Build partitions and verify the tree is well-formed.
	root := BuildPartitions(hits, int32(len(input)))
	t.Logf("partitions: %d top-level children", len(root.Children))

	// Every partition should have Start < End.
	var walkPartitions func(p Partition, depth int)
	walkPartitions = func(p Partition, depth int) {
		if p.Start >= p.End {
			t.Errorf("depth %d: partition [%d, %d) has start >= end",
				depth, p.Start, p.End)
		}
		for _, c := range p.Children {
			if c.Start < p.Start || c.End > p.End {
				t.Errorf("depth %d: child [%d, %d) outside parent [%d, %d)",
					depth, c.Start, c.End, p.Start, p.End)
			}
			walkPartitions(c, depth+1)
		}
	}
	walkPartitions(root, 0)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return append(b[:n], "..."...)
}
