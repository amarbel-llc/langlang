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

// Translate walks a langlang parse tree produced from the CST-mode TOML
// grammar and builds a tommy-compatible *Node tree. The input slice must
// be the same bytes that were parsed to produce the tree.
func Translate(tree langlang.Tree, input []byte) *Node {
	root, ok := tree.Root()
	if !ok {
		return &Node{Kind: NodeDocument}
	}
	return translateNode(tree, input, root)
}

func translateNode(tree langlang.Tree, input []byte, id langlang.NodeID) *Node {
	switch tree.Type(id) {
	case langlang.NodeType_String:
		return makeLeaf(tree, input, id)

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
			// For choices (single child), translate the child directly.
			// For sequences, translate all children.
			if tree.Type(child) == langlang.NodeType_Sequence {
				return &Node{Kind: -1, Children: translateChildren(tree, input, child)}
			}
			// Single child — wrap in a flatten sentinel so it gets inlined.
			inner := translateNode(tree, input, child)
			if inner == nil {
				return nil
			}
			if inner.Kind == -1 {
				return inner // already a flatten sentinel
			}
			return &Node{Kind: -1, Children: []*Node{inner}}
		}

		kind, mapped := nameToKind[name]
		if !mapped {
			return makeLeaf(tree, input, id)
		}

		// Leaf-like named nodes: emit as Raw bytes, no children.
		if leafNames[name] {
			leaf := makeLeaf(tree, input, id)
			if leaf != nil {
				leaf.Kind = kind
			}
			return leaf
		}

		// Container node: recurse into children.
		child, ok := tree.Child(id)
		if !ok {
			return &Node{Kind: kind}
		}
		return &Node{Kind: kind, Children: translateChildren(tree, input, child)}

	case langlang.NodeType_Sequence:
		// Sequence nodes are transparent — inline all children.
		return &Node{Kind: -1, Children: translateChildren(tree, input, id)}

	default:
		return nil
	}
}

func translateChildren(tree langlang.Tree, input []byte, id langlang.NodeID) []*Node {
	var children []*Node
	for _, cid := range tree.Children(id) {
		node := translateNode(tree, input, cid)
		if node == nil {
			continue
		}
		// Flatten sentinel nodes (Kind == -1): inline their children.
		if node.Kind == -1 {
			children = append(children, node.Children...)
		} else {
			children = append(children, node)
		}
	}
	return children
}

func makeLeaf(tree langlang.Tree, input []byte, id langlang.NodeID) *Node {
	span := tree.Span(id)
	start := span.Start.Cursor
	end := span.End.Cursor
	if start >= end || start >= len(input) {
		return nil
	}
	// Use a copy so the Node owns its bytes independent of the input slice.
	raw := make([]byte, end-start)
	copy(raw, input[start:end])
	return &Node{Raw: raw}
}
