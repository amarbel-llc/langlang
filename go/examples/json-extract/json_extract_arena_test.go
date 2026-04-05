package jsonextract

import (
	"testing"
)

// parseAndExtractArena mirrors parseAndExtract but uses arena extraction.
func parseAndExtractArena(t *testing.T, input string) JSONValue {
	t.Helper()
	p := NewJSONParser()
	p.SetInput([]byte(input))
	parsed, err := p.Parse()
	if err != nil {
		t.Fatalf("parse %q: %v", input, err)
	}
	root, ok := parsed.Root()
	if !ok {
		t.Fatalf("no root for %q", input)
	}
	tr := parsed.(*tree)

	var valueID NodeID
	found := false
	tr.Visit(root, func(id NodeID) bool {
		if id == root {
			return true
		}
		if tr.IsNamed(id, _nameID_Value) {
			valueID = id
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatalf("no Value node found in %q", input)
	}

	var a JSONArenas
	c := CountJSONNodes(tr, root)
	a.Alloc(c)

	val, err := ExtractJSONValueArena(tr, valueID, &a)
	if err != nil {
		t.Fatalf("extract arena %q: %v", input, err)
	}
	return *val
}

func TestExtractArenaString(t *testing.T) {
	val := parseAndExtractArena(t, `"hello"`)
	if val.String == nil {
		t.Fatal("expected String to be set")
	}
	if *val.String != `"hello"` {
		t.Errorf("String = %q, want %q", *val.String, `"hello"`)
	}
}

func TestExtractArenaNumber(t *testing.T) {
	val := parseAndExtractArena(t, `42`)
	if val.Number == nil {
		t.Fatal("expected Number to be set")
	}
	if *val.Number != "42" {
		t.Errorf("Number = %q, want %q", *val.Number, "42")
	}
}

func TestExtractArenaObject(t *testing.T) {
	val := parseAndExtractArena(t, `{"name": "test", "count": 42}`)
	if val.Object == nil {
		t.Fatal("expected Object to be set")
	}
	if len(val.Object.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(val.Object.Members))
	}

	m0 := val.Object.Members[0]
	if m0.Key != `"name"` {
		t.Errorf("member[0].Key = %q, want %q", m0.Key, `"name"`)
	}
	if m0.Value.String == nil || *m0.Value.String != `"test"` {
		t.Errorf("member[0].Value.String = %v, want %q", m0.Value.String, `"test"`)
	}

	m1 := val.Object.Members[1]
	if m1.Key != `"count"` {
		t.Errorf("member[1].Key = %q, want %q", m1.Key, `"count"`)
	}
	if m1.Value.Number == nil || *m1.Value.Number != "42" {
		t.Errorf("member[1].Value.Number = %v, want %q", m1.Value.Number, "42")
	}
}

func TestExtractArenaArray(t *testing.T) {
	val := parseAndExtractArena(t, `[1, "two", 3]`)
	if val.Array == nil {
		t.Fatal("expected Array to be set")
	}
	if len(val.Array.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(val.Array.Items))
	}
	if val.Array.Items[0].Number == nil || *val.Array.Items[0].Number != "1" {
		t.Errorf("item[0]: expected Number 1")
	}
	if val.Array.Items[1].String == nil || *val.Array.Items[1].String != `"two"` {
		t.Errorf("item[1]: expected String \"two\"")
	}
	if val.Array.Items[2].Number == nil || *val.Array.Items[2].Number != "3" {
		t.Errorf("item[2]: expected Number 3")
	}
}

func TestExtractArenaNested(t *testing.T) {
	val := parseAndExtractArena(t, `{"items": [1, 2]}`)
	if val.Object == nil {
		t.Fatal("expected Object")
	}
	if len(val.Object.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(val.Object.Members))
	}
	m := val.Object.Members[0]
	if m.Value.Array == nil {
		t.Fatal("expected nested Array")
	}
	if len(m.Value.Array.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(m.Value.Array.Items))
	}
}

func TestExtractArenaDeeplyNested(t *testing.T) {
	depth := 100
	var b []byte
	for range depth {
		b = append(b, `{"k": `...)
	}
	b = append(b, `[1, "x"]`...)
	for range depth {
		b = append(b, '}')
	}

	val := parseAndExtractArena(t, string(b))

	current := val
	for i := range depth {
		if current.Object == nil {
			t.Fatalf("depth %d: expected Object", i)
		}
		if len(current.Object.Members) != 1 {
			t.Fatalf("depth %d: expected 1 member, got %d", i, len(current.Object.Members))
		}
		current = current.Object.Members[0].Value
	}

	if current.Array == nil {
		t.Fatal("leaf: expected Array")
	}
	if len(current.Array.Items) != 2 {
		t.Fatalf("leaf: expected 2 items, got %d", len(current.Array.Items))
	}
}
