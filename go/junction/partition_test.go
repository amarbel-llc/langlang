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

func TestBuildPartitionsUnmatchedOpen(t *testing.T) {
	t.Run("trailing unclosed bracket", func(t *testing.T) {
		// {"key": [1, 2, 3  — missing ]}
		input := []byte(`{"key": [1, 2, 3`)
		hits := ScanJunctions(input, jsonSpec)
		inputLen := int32(len(input))
		root := BuildPartitions(hits, inputLen)

		if len(root.Children) != 1 {
			t.Fatalf("root children = %d, want 1", len(root.Children))
		}

		obj := root.Children[0]
		// The { is also unclosed, so it should extend to EOF.
		if obj.End != inputLen {
			t.Errorf("unclosed { partition End = %d, want %d", obj.End, inputLen)
		}

		if len(obj.Children) != 1 {
			t.Fatalf("object children = %d, want 1", len(obj.Children))
		}

		arr := obj.Children[0]
		if arr.End != inputLen {
			t.Errorf("unclosed [ partition End = %d, want %d", arr.End, inputLen)
		}
	})

	t.Run("mismatched nesting", func(t *testing.T) {
		// {[} — stack-based close is byte-blind, so } closes [ (top
		// of stack), leaving { on the stack for the post-loop fixup.
		input := []byte(`{[}`)
		hits := ScanJunctions(input, jsonSpec)
		inputLen := int32(len(input))
		root := BuildPartitions(hits, inputLen)

		if len(root.Children) != 1 {
			t.Fatalf("root children = %d, want 1", len(root.Children))
		}

		obj := root.Children[0]
		// { is never matched by its own close — post-loop fixup sets End=inputLen.
		if obj.End != inputLen {
			t.Errorf("{ partition End = %d, want %d", obj.End, inputLen)
		}

		if len(obj.Children) != 1 {
			t.Fatalf("{ children = %d, want 1", len(obj.Children))
		}

		// } closes [ (byte-blind stack pop), so [ gets End=3.
		bracket := obj.Children[0]
		if bracket.End != 3 {
			t.Errorf("[ partition End = %d, want 3", bracket.End)
		}
	})

	t.Run("close before any open", func(t *testing.T) {
		// Leading } with no matching { — should be ignored, no children.
		input := []byte(`}hello`)
		hits := ScanJunctions(input, jsonSpec)
		inputLen := int32(len(input))
		root := BuildPartitions(hits, inputLen)

		if root.End != inputLen {
			t.Errorf("root End = %d, want %d", root.End, inputLen)
		}
		// The stray } should not create any partitions.
		if len(root.Children) != 0 {
			t.Errorf("root children = %d, want 0", len(root.Children))
		}
	})

	t.Run("multiple unclosed at different depths", func(t *testing.T) {
		// {[( — three opens, no closes
		spec := ScannerSpec{
			Junctions: []JunctionByte{
				{'{', JunctionOpen},
				{'[', JunctionOpen},
				{'(', JunctionOpen},
				{'}', JunctionClose},
				{']', JunctionClose},
				{')', JunctionClose},
			},
		}
		input := []byte(`{[(abc`)
		hits := ScanJunctions(input, spec)
		inputLen := int32(len(input))
		root := BuildPartitions(hits, inputLen)

		if len(root.Children) != 1 {
			t.Fatalf("root children = %d, want 1", len(root.Children))
		}

		d0 := root.Children[0] // {
		if d0.End != inputLen {
			t.Errorf("unclosed { End = %d, want %d", d0.End, inputLen)
		}
		if len(d0.Children) != 1 {
			t.Fatalf("{ children = %d, want 1", len(d0.Children))
		}

		d1 := d0.Children[0] // [
		if d1.End != inputLen {
			t.Errorf("unclosed [ End = %d, want %d", d1.End, inputLen)
		}
		if len(d1.Children) != 1 {
			t.Fatalf("[ children = %d, want 1", len(d1.Children))
		}

		d2 := d1.Children[0] // (
		if d2.End != inputLen {
			t.Errorf("unclosed ( End = %d, want %d", d2.End, inputLen)
		}
	})

	t.Run("only excess closes", func(t *testing.T) {
		// }] — two closes with no opens
		input := []byte(`}]`)
		hits := ScanJunctions(input, jsonSpec)
		inputLen := int32(len(input))
		root := BuildPartitions(hits, inputLen)

		if root.End != inputLen {
			t.Errorf("root End = %d, want %d", root.End, inputLen)
		}
		if len(root.Children) != 0 {
			t.Errorf("root children = %d, want 0", len(root.Children))
		}
	})

	t.Run("valid prefix then unclosed", func(t *testing.T) {
		// [1,2],[3 — first bracket matched, second unclosed
		input := []byte(`[1,2],[3`)
		hits := ScanJunctions(input, jsonSpec)
		inputLen := int32(len(input))
		root := BuildPartitions(hits, inputLen)

		if len(root.Children) != 2 {
			t.Fatalf("root children = %d, want 2", len(root.Children))
		}

		closed := root.Children[0]
		if closed.End != 5 {
			t.Errorf("closed [ End = %d, want 5", closed.End)
		}

		unclosed := root.Children[1]
		if unclosed.End != inputLen {
			t.Errorf("unclosed [ End = %d, want %d", unclosed.End, inputLen)
		}
	})

	t.Run("unclosed with closed sibling inside", func(t *testing.T) {
		// {[1,2] — the [ is properly closed but { is not
		input := []byte(`{[1,2]`)
		hits := ScanJunctions(input, jsonSpec)
		inputLen := int32(len(input))
		root := BuildPartitions(hits, inputLen)

		if len(root.Children) != 1 {
			t.Fatalf("root children = %d, want 1", len(root.Children))
		}

		obj := root.Children[0]
		if obj.End != inputLen {
			t.Errorf("unclosed { End = %d, want %d", obj.End, inputLen)
		}

		// The [ inside should be properly closed at pos 5 (the ]).
		if len(obj.Children) != 1 {
			t.Fatalf("{ children = %d, want 1", len(obj.Children))
		}
		arr := obj.Children[0]
		if arr.End != 6 {
			t.Errorf("closed [ End = %d, want 6", arr.End)
		}
	})
}
