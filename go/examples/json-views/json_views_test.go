package jsonviews

import (
	"testing"
)

func parseJSON(t *testing.T, input string) JSONView {
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
	return newJSONView(parsed.(*tree), root)
}

func TestViewString(t *testing.T) {
	json := parseJSON(t, `"hello"`)
	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value child")
	}
	str, ok := val.String()
	if !ok {
		t.Fatal("expected String alternative")
	}
	if str.Text() != `"hello"` {
		t.Errorf("String.Text() = %q, want %q", str.Text(), `"hello"`)
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
	if num.Text() != "42" {
		t.Errorf("Number.Text() = %q, want %q", num.Text(), "42")
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

	mem, ok := obj.Member()
	if !ok {
		t.Fatal("no Member in Object")
	}
	key, ok := mem.String()
	if !ok {
		t.Fatal("no String in Member")
	}
	if key.Text() != `"name"` {
		t.Errorf("Member key = %q, want %q", key.Text(), `"name"`)
	}

	mval, ok := mem.Value()
	if !ok {
		t.Fatal("no Value in Member")
	}
	str, ok := mval.String()
	if !ok {
		t.Fatal("expected String value")
	}
	if str.Text() != `"test"` {
		t.Errorf("Member value = %q, want %q", str.Text(), `"test"`)
	}
}

func TestViewArray(t *testing.T) {
	json := parseJSON(t, `[1, 2, 3]`)
	val, ok := json.Value()
	if !ok {
		t.Fatal("no Value child")
	}
	arr, ok := val.Array()
	if !ok {
		t.Fatal("expected Array alternative")
	}
	// With current classifier, Value appears as a single child.
	// The first Value in the array is accessible.
	v, ok := arr.Value()
	if !ok {
		t.Fatal("no Value in Array")
	}
	num, ok := v.Number()
	if !ok {
		t.Fatal("expected Number")
	}
	if num.Text() != "1" {
		t.Errorf("first item = %q, want %q", num.Text(), "1")
	}
}

func TestViewText(t *testing.T) {
	json := parseJSON(t, `{"a": 1}`)
	if json.Text() != `{"a": 1}` {
		t.Errorf("JSON.Text() = %q, want %q", json.Text(), `{"a": 1}`)
	}
}
