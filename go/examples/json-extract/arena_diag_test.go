package jsonextract

import (
	"testing"
)

// BenchmarkCountOnly measures just the tree-walk pre-count pass.
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
				_ = countNodes(tr, root)
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

// BenchmarkExtractOnlyArena measures extraction without parsing (arena, tree-walk count).
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
			if tr.IsNamed(id, _arenaNameID_Value) {
				valueID = id
				return false
			}
			return true
		})

		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			var a JsonextractArenas
			for b.Loop() {
				c := countNodes(tr, root)
				a.Alloc(c)
				_, err := ExtractJSONValueArena(tr, valueID, &a)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
