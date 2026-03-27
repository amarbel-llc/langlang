package junction

import (
	"testing"
)

func TestBuildPartitionsBasic(t *testing.T) {
	// {"a":1,"b":[2,3]}
	input := []byte(`{"a":1,"b":[2,3]}`)
	hits := ScanJunctions(input, jsonSpec)
	root := BuildPartitions(hits, int32(len(input)))

	if root.Start != 0 || root.End != int32(len(input)) {
		t.Fatalf("root = [%d,%d), want [0,%d)", root.Start, root.End, len(input))
	}

	// Root should have one child: the outer object {…}
	if len(root.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(root.Children))
	}

	obj := root.Children[0]
	if obj.Start != 0 || obj.End != 17 || obj.Depth != 0 {
		t.Errorf("object = {Start:%d End:%d Depth:%d}, want {0 17 0}", obj.Start, obj.End, obj.Depth)
	}

	// Object should have one child: the array [2,3]
	if len(obj.Children) != 1 {
		t.Fatalf("object children = %d, want 1", len(obj.Children))
	}

	arr := obj.Children[0]
	if arr.Start != 11 || arr.End != 16 || arr.Depth != 1 {
		t.Errorf("array = {Start:%d End:%d Depth:%d}, want {11 16 1}", arr.Start, arr.End, arr.Depth)
	}

	if len(arr.Children) != 0 {
		t.Errorf("array children = %d, want 0", len(arr.Children))
	}
}

func TestBuildPartitionsNested(t *testing.T) {
	// {"a":{"b":{"c":1}}}
	input := []byte(`{"a":{"b":{"c":1}}}`)
	hits := ScanJunctions(input, jsonSpec)
	root := BuildPartitions(hits, int32(len(input)))

	if len(root.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(root.Children))
	}

	// Depth 0 object
	d0 := root.Children[0]
	if d0.Depth != 0 || len(d0.Children) != 1 {
		t.Fatalf("d0: depth=%d children=%d, want depth=0 children=1", d0.Depth, len(d0.Children))
	}

	// Depth 1 object
	d1 := d0.Children[0]
	if d1.Depth != 1 || len(d1.Children) != 1 {
		t.Fatalf("d1: depth=%d children=%d, want depth=1 children=1", d1.Depth, len(d1.Children))
	}

	// Depth 2 object
	d2 := d1.Children[0]
	if d2.Depth != 2 || len(d2.Children) != 0 {
		t.Errorf("d2: depth=%d children=%d, want depth=2 children=0", d2.Depth, len(d2.Children))
	}
}

func TestBuildPartitionsEmpty(t *testing.T) {
	root := BuildPartitions(nil, 0)
	if root.Start != 0 || root.End != 0 || len(root.Children) != 0 {
		t.Errorf("empty partition = %+v, want zero", root)
	}
}
