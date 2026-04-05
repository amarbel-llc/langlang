package tomlcst

import langlang "github.com/clarete/langlang/go"

// nameToKind maps langlang grammar rule names to CST NodeKind values.
var nameToKind = map[string]NodeKind{
	"TOML":            NodeDocument,
	"StdTable":        NodeTable,
	"ArrayTable":      NodeArrayTable,
	"KeyVal":          NodeKeyValue,
	"InlineKeyVal":    NodeKeyValue,
	"Key":             NodeKey,
	"Equals":          NodeEquals,
	"Dot":             NodeDot,
	"Comma":           NodeComma,
	"BracketOpen":     NodeBracketOpen,
	"BracketClose":    NodeBracketClose,
	"BraceOpen":       NodeBraceOpen,
	"BraceClose":      NodeBraceClose,
	"WS":              NodeWhitespace,
	"Newline":         NodeNewline,
	"Comment":         NodeComment,
	"BasicString":     NodeString,
	"MLBasicString":   NodeString,
	"LiteralString":   NodeString,
	"MLLiteralString": NodeString,
	"Integer":         NodeInteger,
	"Float":           NodeFloat,
	"Boolean":         NodeBool,
	"DateTime":        NodeDateTime,
	"Array":           NodeArray,
	"InlineTable":     NodeInlineTable,
}

// flattenNames lists rule names whose children should be inlined into the
// parent rather than creating a wrapper node.
var flattenNames = map[string]bool{
	"Trivia":             true,
	"LineEnd":            true,
	"Expression":         true,
	"Val":                true,
	"SimpleKey":          true,
	"QuotedKey":          true,
	"InlineTableKeyVals": true,
	"ArrayValues":        true,
}

// leafNames lists rule names that should produce leaf nodes with Raw bytes
// rather than container nodes with children.
var leafNames = map[string]bool{
	"WS": true, "Newline": true, "Comment": true,
	"Equals": true, "Dot": true, "Comma": true,
	"BracketOpen": true, "BracketClose": true,
	"BraceOpen": true, "BraceClose": true,
	"UnquotedKey": true,
	"BasicString": true, "MLBasicString": true,
	"LiteralString": true, "MLLiteralString": true,
	"Integer": true, "Float": true, "Boolean": true, "DateTime": true,
}

// skipNames lists rule names that produce no CST output.
var skipNames = map[string]bool{
	"EOF":     true,
	"Spacing": true,
	"Space":   true,
	"EOL":     true,
}

// arena pre-allocates Node and []*Node storage to reduce heap allocations
// during tree translation. Nodes are allocated from a flat slice; child
// pointer slices share a single backing array.
type arena struct {
	nodes   []Node
	nUsed   int
	ptrs    []*Node
	pUsed   int
	scratch []*Node // reusable buffer for translateChildren
}

func newArena(nodeCount, ptrCount int) *arena {
	a := &arena{
		nodes: make([]Node, nodeCount),
		ptrs:  make([]*Node, ptrCount),
	}
	return a
}

func (a *arena) allocNode() *Node {
	if a.nUsed >= len(a.nodes) {
		a.nodes = append(a.nodes, Node{})
	}
	n := &a.nodes[a.nUsed]
	a.nUsed++
	return n
}

func (a *arena) allocChildren(count int) []*Node {
	if a.pUsed+count > len(a.ptrs) {
		need := a.pUsed + count - len(a.ptrs)
		a.ptrs = append(a.ptrs, make([]*Node, need)...)
	}
	s := a.ptrs[a.pUsed : a.pUsed+count]
	a.pUsed += count
	return s
}

// Translate walks a langlang parse tree produced from the CST-mode TOML
// grammar and builds a tommy-compatible *Node tree. The input slice must
// be the same bytes that were parsed to produce the tree.
//
// Nodes are allocated from a pre-sized arena to minimize heap allocations.
// Leaf nodes reference the input slice directly (no copy).
//
// The tree is type-asserted to *ConcreteTree so that IterDirectChildren
// calls are inlined by the compiler (avoiding interface dispatch and
// closure heap allocation).
func Translate(tree langlang.Tree, input []byte) *Node {
	ct, ok := tree.(*langlang.ConcreteTree)
	if !ok {
		return &Node{Kind: NodeDocument}
	}
	root, ok := ct.Root()
	if !ok {
		return &Node{Kind: NodeDocument}
	}
	// Estimate arena size from input length. TOML CST produces roughly
	// 1 node per 3 bytes (tokens + trivia).
	est := len(input) / 3
	if est < 64 {
		est = 64
	}
	a := newArena(est, est*3)
	return translateNode(a, ct, input, root)
}

func translateNode(a *arena, tree *langlang.ConcreteTree, input []byte, id langlang.NodeID) *Node {
	switch tree.Type(id) {
	case langlang.NodeType_String:
		return makeLeaf(a, tree, input, id)

	case langlang.NodeType_Node:
		name := tree.Name(id)

		if skipNames[name] {
			return nil
		}

		if flattenNames[name] {
			child, ok := tree.Child(id)
			if !ok {
				return nil
			}
			if tree.Type(child) == langlang.NodeType_Sequence {
				n := a.allocNode()
				n.Kind = -1
				n.Children = translateChildren(a, tree, input, child)
				return n
			}
			inner := translateNode(a, tree, input, child)
			if inner == nil {
				return nil
			}
			if inner.Kind == -1 {
				return inner
			}
			n := a.allocNode()
			n.Kind = -1
			n.Children = a.allocChildren(1)
			n.Children[0] = inner
			return n
		}

		kind, mapped := nameToKind[name]
		if !mapped {
			return makeLeaf(a, tree, input, id)
		}

		if leafNames[name] {
			leaf := makeLeaf(a, tree, input, id)
			if leaf != nil {
				leaf.Kind = kind
			}
			return leaf
		}

		child, ok := tree.Child(id)
		if !ok {
			n := a.allocNode()
			n.Kind = kind
			return n
		}
		n := a.allocNode()
		n.Kind = kind
		n.Children = translateChildren(a, tree, input, child)
		return n

	case langlang.NodeType_Sequence:
		n := a.allocNode()
		n.Kind = -1
		n.Children = translateChildren(a, tree, input, id)
		return n

	default:
		return nil
	}
}

func translateChildren(a *arena, tree *langlang.ConcreteTree, input []byte, id langlang.NodeID) []*Node {
	// Use mark/restore on the scratch buffer so recursive calls don't
	// clobber the parent's collected children.
	mark := len(a.scratch)
	for cid := range tree.IterDirectChildren(id) {
		node := translateNode(a, tree, input, cid)
		if node == nil {
			continue
		}
		if node.Kind == -1 {
			a.scratch = append(a.scratch, node.Children...)
		} else {
			a.scratch = append(a.scratch, node)
		}
	}
	count := len(a.scratch) - mark
	if count == 0 {
		a.scratch = a.scratch[:mark]
		return nil
	}
	children := a.allocChildren(count)
	copy(children, a.scratch[mark:])
	a.scratch = a.scratch[:mark]
	return children
}

func makeLeaf(a *arena, tree *langlang.ConcreteTree, input []byte, id langlang.NodeID) *Node {
	span := tree.Span(id)
	start := span.Start.Cursor
	end := span.End.Cursor
	if start >= end || start >= len(input) {
		return nil
	}
	n := a.allocNode()
	// Reference input directly — no copy. The input must outlive the tree.
	n.Raw = input[start:end]
	return n
}
