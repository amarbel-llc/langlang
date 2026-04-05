package jsonextract

import (
	"testing"
)

// BenchmarkCountOnly measures just the pre-count pass.
func BenchmarkCountOnly(b *testing.B) {
	inputs := benchInputs(b)
	p := NewJSONParser()
	p.SetShowFails(false)

	for _, name := range inputNames {
		input := inputs[name]
		p.SetInput(input)
		parsed, err := p.ParseJSON()
		if err != nil {
			b.Fatal(err)
		}
		tr := parsed.(*tree)
		root, _ := parsed.Root()

		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			for b.Loop() {
				_ = CountJSONNodes(tr, root)
			}
		})
	}
}

// BenchmarkAllocOnly measures pre-count + arena allocation (no extraction).
func BenchmarkAllocOnly(b *testing.B) {
	inputs := benchInputs(b)
	p := NewJSONParser()
	p.SetShowFails(false)

	for _, name := range inputNames {
		input := inputs[name]
		p.SetInput(input)
		parsed, err := p.ParseJSON()
		if err != nil {
			b.Fatal(err)
		}
		tr := parsed.(*tree)
		root, _ := parsed.Root()

		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			var a JSONArenas
			for b.Loop() {
				c := CountJSONNodes(tr, root)
				a.Alloc(c)
			}
		})
	}
}

// BenchmarkExtractOnlyHeap measures extraction without parsing (heap).
func BenchmarkExtractOnlyHeap(b *testing.B) {
	inputs := benchInputs(b)
	p := NewJSONParser()
	p.SetShowFails(false)

	for _, name := range inputNames {
		input := inputs[name]
		p.SetInput(input)
		parsed, err := p.ParseJSON()
		if err != nil {
			b.Fatal(err)
		}
		tr := parsed.(*tree)
		root, _ := parsed.Root()

		var valueID NodeID
		tr.Visit(root, func(id NodeID) bool {
			if id == root {
				return true
			}
			if tr.IsNamed(id, _nameID_Value) {
				valueID = id
				return false
			}
			return true
		})

		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			for b.Loop() {
				_, err := ExtractJSONValue(tr, valueID)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkExtractOnlyArena measures extraction without parsing (arena).
func BenchmarkExtractOnlyArena(b *testing.B) {
	inputs := benchInputs(b)
	p := NewJSONParser()
	p.SetShowFails(false)

	for _, name := range inputNames {
		input := inputs[name]
		p.SetInput(input)
		parsed, err := p.ParseJSON()
		if err != nil {
			b.Fatal(err)
		}
		tr := parsed.(*tree)
		root, _ := parsed.Root()

		var valueID NodeID
		tr.Visit(root, func(id NodeID) bool {
			if id == root {
				return true
			}
			if tr.IsNamed(id, _nameID_Value) {
				valueID = id
				return false
			}
			return true
		})

		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			var a JSONArenas
			for b.Loop() {
				c := CountJSONNodes(tr, root)
				a.Alloc(c)
				_, err := ExtractJSONValueArena(tr, valueID, &a)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestAllocSources counts where arena extraction allocations come from.
func TestAllocSources(t *testing.T) {
	input := []byte(`{"a": [1, 2], "b": {"c": "d"}}`)
	p := NewJSONParser()
	p.SetInput(input)
	parsed, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	tr := parsed.(*tree)
	root, _ := parsed.Root()

	var valueID NodeID
	tr.Visit(root, func(id NodeID) bool {
		if id == root {
			return true
		}
		if tr.IsNamed(id, _nameID_Value) {
			valueID = id
			return false
		}
		return true
	})

	var a JSONArenas
	c := CountJSONNodes(tr, root)
	a.Alloc(c)
	t.Logf("counts: values=%d objects=%d members=%d arrays=%d strings=%d",
		c.Values, c.Objects, c.Members, c.Arrays, c.Strings)

	_, err = ExtractJSONValueArena(tr, valueID, &a)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("arena usage: values=%d/%d objects=%d/%d members=%d/%d arrays=%d/%d",
		len(a.Values), cap(a.Values),
		len(a.Objects), cap(a.Objects),
		len(a.Members), cap(a.Members),
		len(a.Arrays), cap(a.Arrays))
}
