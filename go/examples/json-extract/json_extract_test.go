package jsonextract

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func parseAndExtract(t *testing.T, input string) JSONValue {
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

	// Root is JSON node. Navigate: JSON -> Value (its child's child,
	// since JSON is NodeType_Node wrapping a sequence/child).
	// Find the Value node by walking children.
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

	val, err := ExtractJSONValue(tr, valueID)
	if err != nil {
		t.Fatalf("extract %q: %v", input, err)
	}
	return val
}

func TestExtractString(t *testing.T) {
	val := parseAndExtract(t, `"hello"`)
	if val.String == nil {
		t.Fatal("expected String to be set")
	}
	// The String rule captures the full match including quotes
	// based on the grammar: String <- '"' #(Char* '"')
	got := *val.String
	if got != `"hello"` {
		t.Errorf("String = %q, want %q", got, `"hello"`)
	}
}

func TestExtractNumber(t *testing.T) {
	val := parseAndExtract(t, `42`)
	if val.Number == nil {
		t.Fatal("expected Number to be set")
	}
	if *val.Number != "42" {
		t.Errorf("Number = %q, want %q", *val.Number, "42")
	}
}

func TestExtractObject(t *testing.T) {
	val := parseAndExtract(t, `{"name": "test", "count": 42}`)
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
	if m0.Value.String == nil {
		t.Error("member[0].Value.String not set")
	} else if *m0.Value.String != `"test"` {
		t.Errorf("member[0].Value.String = %q, want %q", *m0.Value.String, `"test"`)
	}

	m1 := val.Object.Members[1]
	if m1.Key != `"count"` {
		t.Errorf("member[1].Key = %q, want %q", m1.Key, `"count"`)
	}
	if m1.Value.Number == nil {
		t.Error("member[1].Value.Number not set")
	} else if *m1.Value.Number != "42" {
		t.Errorf("member[1].Value.Number = %q, want %q", *m1.Value.Number, "42")
	}
}

func TestExtractArray(t *testing.T) {
	val := parseAndExtract(t, `[1, "two", 3]`)
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

func TestExtractNested(t *testing.T) {
	val := parseAndExtract(t, `{"items": [1, 2]}`)
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

// countExtracted recursively walks extracted structs and counts nodes.
type extractCounts struct {
	objects int
	arrays  int
	strings int
	numbers int
	empty   int // literals (true/false/null) — not captured by struct extraction
	members int
}

func countExtractedValue(v JSONValue, c *extractCounts) {
	switch {
	case v.Object != nil:
		c.objects++
		for _, m := range v.Object.Members {
			c.members++
			c.strings++ // key is always a string
			countExtractedValue(m.Value, c)
		}
	case v.Array != nil:
		c.arrays++
		for _, item := range v.Array.Items {
			countExtractedValue(item, c)
		}
	case v.String != nil:
		c.strings++
	case v.Number != nil:
		c.numbers++
	default:
		c.empty++ // literal: true, false, or null
	}
}

func TestExtractLargeDocument(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "testdata", "json")

	for _, name := range []string{"30kb", "500kb"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(base, "input_"+name+".json"))
			if err != nil {
				t.Skipf("test data not available: %v", err)
			}

			p := NewJSONParser()
			p.SetShowFails(false)
			p.SetInput(data)
			parsed, err := p.ParseJSON()
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			root, ok := parsed.Root()
			if !ok {
				t.Fatal("no root")
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
				t.Fatal("no Value node")
			}

			val, err := ExtractJSONValue(tr, valueID)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}

			var c extractCounts
			countExtractedValue(val, &c)

			t.Logf("objects=%d arrays=%d strings=%d numbers=%d literals=%d members=%d",
				c.objects, c.arrays, c.strings, c.numbers, c.empty, c.members)

			total := c.objects + c.arrays + c.strings + c.numbers + c.empty
			if total == 0 {
				t.Error("walked zero values")
			}
			if c.objects > 0 && c.members == 0 {
				t.Error("objects found but no members")
			}
			if c.members > 0 && c.strings == 0 {
				t.Error("members found but no strings")
			}
		})
	}
}

func TestExtractDeeplyNested(t *testing.T) {
	depth := 100
	var b []byte
	for range depth {
		b = append(b, `{"k": `...)
	}
	b = append(b, `[1, "x"]`...)
	for range depth {
		b = append(b, '}')
	}

	val := parseAndExtract(t, string(b))

	// Walk down nested objects.
	current := val
	for i := range depth {
		if current.Object == nil {
			t.Fatalf("depth %d: expected Object", i)
		}
		if len(current.Object.Members) != 1 {
			t.Fatalf("depth %d: expected 1 member, got %d", i, len(current.Object.Members))
		}
		m := current.Object.Members[0]
		if m.Key != `"k"` {
			t.Fatalf("depth %d: key = %q, want %q", i, m.Key, `"k"`)
		}
		current = m.Value
	}

	// At the leaf: [1, "x"]
	if current.Array == nil {
		t.Fatal("leaf: expected Array")
	}
	if len(current.Array.Items) != 2 {
		t.Fatalf("leaf: expected 2 items, got %d", len(current.Array.Items))
	}
	if current.Array.Items[0].Number == nil || *current.Array.Items[0].Number != "1" {
		t.Error("leaf item 0: expected Number 1")
	}
	if current.Array.Items[1].String == nil || *current.Array.Items[1].String != `"x"` {
		t.Error("leaf item 1: expected String \"x\"")
	}
}
