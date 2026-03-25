package jsonviews

import (
	"fmt"
	"testing"
)

func parseJSON(t *testing.T, input string) JSON {
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
	return newJSON(parsed.(*tree), root)
}

func TestViewString(t *testing.T) {
	json := parseJSON(t, `"hello"`)
	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value child")
	}
	str, ok := val.StringNode()
	if !ok {
		t.Fatal("expected String alternative")
	}
	if str.String() != `"hello"` {
		t.Errorf("String.String() = %q, want %q", str.String(), `"hello"`)
	}
}

func TestViewNumber(t *testing.T) {
	json := parseJSON(t, `42`)
	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value child")
	}
	num, ok := val.Number()
	if !ok {
		t.Fatal("expected Number alternative")
	}
	if num.String() != "42" {
		t.Errorf("Number.String() = %q, want %q", num.String(), "42")
	}
}

func TestViewObject(t *testing.T) {
	json := parseJSON(t, `{"name": "test"}`)

	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value child")
	}
	obj, ok := val.Object()
	if !ok {
		t.Fatal("expected Object alternative")
	}

	if obj.MemberCount() == 0 {
		t.Fatal("no Member in Object")
	}
	mem := obj.MemberAt(0)
	key, ok := mem.StringNode()
	if !ok {
		t.Fatal("no String in Member")
	}
	if key.String() != `"name"` {
		t.Errorf("Member key = %q, want %q", key.String(), `"name"`)
	}

	mval, ok := mem.Value()
	if !ok {
		t.Fatal("no Value in Member")
	}
	str, ok := mval.StringNode()
	if !ok {
		t.Fatal("expected String value")
	}
	if str.String() != `"test"` {
		t.Errorf("Member value = %q, want %q", str.String(), `"test"`)
	}
}

func TestViewArrayAllValues(t *testing.T) {
	json := parseJSON(t, `[1, 2, 3]`)
	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value child")
	}
	arr, ok := val.Array()
	if !ok {
		t.Fatal("expected Array alternative")
	}

	// Array should expose all values, not just the first.
	if arr.ValueCount() != 3 {
		t.Fatalf("ValueCount() = %d, want 3", arr.ValueCount())
	}

	want := []string{"1", "2", "3"}
	for i, w := range want {
		v := arr.ValueAt(i)
		num, ok := v.Number()
		if !ok {
			t.Fatalf("item %d: expected Number", i)
		}
		if num.String() != w {
			t.Errorf("item %d = %q, want %q", i, num.String(), w)
		}
	}
}

func TestViewObjectAllMembers(t *testing.T) {
	json := parseJSON(t, `{"a": 1, "b": 2}`)
	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value child")
	}
	obj, ok := val.Object()
	if !ok {
		t.Fatal("expected Object alternative")
	}

	// Object should expose all members, not just the first.
	if obj.MemberCount() != 2 {
		t.Fatalf("MemberCount() = %d, want 2", obj.MemberCount())
	}

	wantKeys := []string{`"a"`, `"b"`}
	for i, wk := range wantKeys {
		mem := obj.MemberAt(i)
		key, ok := mem.StringNode()
		if !ok {
			t.Fatalf("member %d: no String key", i)
		}
		if key.String() != wk {
			t.Errorf("member %d key = %q, want %q", i, key.String(), wk)
		}
	}
}

func TestViewPublicConstructor(t *testing.T) {
	p := NewJSONParser()
	p.SetInput([]byte(`42`))
	parsed, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	// Users should be able to create a root view without casting to *tree.
	json := NewJSON(parsed)
	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value child")
	}
	num, ok := val.Number()
	if !ok {
		t.Fatal("expected Number")
	}
	if num.String() != "42" {
		t.Errorf("got %q, want %q", num.String(), "42")
	}
}

func TestViewMemberFmtStringer(t *testing.T) {
	json := parseJSON(t, `{"key": "val"}`)
	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value")
	}
	obj, ok := val.Object()
	if !ok {
		t.Fatal("no Object")
	}
	mem := obj.MemberAt(0)

	// Member should be usable with fmt.Sprintf("%s") without compile error.
	// This tests that Member satisfies fmt.Stringer (has String() string).
	s := fmt.Sprintf("%s", mem)
	if s != `"key": "val"` {
		t.Errorf("fmt.Sprintf(\"%%s\", member) = %q, want %q", s, `"key": "val"`)
	}
}

func TestViewText(t *testing.T) {
	json := parseJSON(t, `{"a": 1}`)
	if json.String() != `{"a": 1}` {
		t.Errorf("JSON.String() = %q, want %q", json.String(), `{"a": 1}`)
	}
}
