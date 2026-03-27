package junction

import (
	"testing"
)

func TestIntegrationJSON(t *testing.T) {
	// End-to-end: grammar -> scanner spec -> scan -> partitions.
	spec, err := AnalyzeForJunctions(jsonGrammarPath())
	if err != nil {
		t.Fatalf("AnalyzeForJunctions: %v", err)
	}

	input := []byte(`{"name":"alice","items":[1,2,{"x":3}]}`)
	hits := ScanJunctions(input, spec)

	// Verify the structural skeleton matches the reference doc example.
	type expected struct {
		pos   int32
		depth int16
		kind  JunctionKind
		b     byte
	}

	// {"name":"alice","items":[1,2,{"x":3}]}
	// 0123456789...
	want := []expected{
		{0, 0, JunctionOpen, '{'},
		{7, 1, JunctionSeparator, ':'},
		{15, 1, JunctionSeparator, ','},
		{23, 1, JunctionSeparator, ':'},
		{24, 1, JunctionOpen, '['},
		{26, 2, JunctionSeparator, ','},
		{28, 2, JunctionSeparator, ','},
		{29, 2, JunctionOpen, '{'},
		{33, 3, JunctionSeparator, ':'},
		{35, 2, JunctionClose, '}'},
		{36, 1, JunctionClose, ']'},
		{37, 0, JunctionClose, '}'},
	}

	if len(hits) != len(want) {
		t.Fatalf("got %d hits, want %d\nhits: %+v", len(hits), len(want), hits)
	}
	for i, got := range hits {
		w := want[i]
		if got.Pos != w.pos || got.Depth != w.depth || got.Kind != w.kind || got.Byte != w.b {
			t.Errorf("hit[%d] = {Pos:%d Depth:%d Kind:%d Byte:%c}, want {Pos:%d Depth:%d Kind:%d Byte:%c}",
				i, got.Pos, got.Depth, got.Kind, got.Byte, w.pos, w.depth, w.kind, w.b)
		}
	}

	// Build partitions and verify structure.
	root := BuildPartitions(hits, int32(len(input)))

	if root.Start != 0 || root.End != int32(len(input)) {
		t.Fatalf("root = [%d,%d), want [0,%d)", root.Start, root.End, len(input))
	}

	// Root has one child: the outer object.
	if len(root.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(root.Children))
	}

	obj := root.Children[0]
	if obj.Depth != 0 {
		t.Errorf("object depth = %d, want 0", obj.Depth)
	}

	// Object should have one child: the array (which in turn contains the nested object).
	// Array [1,2,{"x":3}] starts at 24, nested object starts at 29.
	if len(obj.Children) != 1 {
		t.Fatalf("object children = %d, want 1 (the array)", len(obj.Children))
	}

	arr := obj.Children[0]
	if arr.Depth != 1 || input[arr.Start] != '[' {
		t.Errorf("array: depth=%d start_byte=%c, want depth=1 start_byte=[", arr.Depth, input[arr.Start])
	}

	// Array has one child: the nested object {"x":3}.
	if len(arr.Children) != 1 {
		t.Fatalf("array children = %d, want 1 (nested object)", len(arr.Children))
	}

	nested := arr.Children[0]
	if nested.Depth != 2 || input[nested.Start] != '{' {
		t.Errorf("nested: depth=%d start_byte=%c, want depth=2 start_byte={", nested.Depth, input[nested.Start])
	}
}
