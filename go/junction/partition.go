package junction

// BuildPartitions constructs a hierarchical partition tree from junction hits.
// Each matched Open/Close pair becomes a child partition of its enclosing scope.
func BuildPartitions(hits []JunctionHit, inputLen int32) Partition {
	root := Partition{Start: 0, End: inputLen, Depth: -1}
	stack := []*Partition{&root}

	for _, h := range hits {
		switch h.Kind {
		case JunctionOpen:
			p := Partition{Start: h.Pos, Depth: h.Depth}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, p)
			// Push pointer to the just-appended child.
			stack = append(stack, &parent.Children[len(parent.Children)-1])

		case JunctionClose:
			if len(stack) > 1 {
				stack[len(stack)-1].End = h.Pos + 1
				stack = stack[:len(stack)-1]
			}
		}
	}

	return root
}
