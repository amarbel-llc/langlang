// Package tomlcst provides a mutable concrete syntax tree for TOML documents.
// It is designed to be compatible with tommy's CST representation, enabling
// format-preserving round-trip editing.
//
// The Node type mirrors tommy's pkg/cst.Node exactly: leaf nodes store raw
// bytes, container nodes have children, and Bytes() recursively concatenates
// to reproduce the original input byte-for-byte.
package tomlcst

// NodeKind classifies CST nodes. The enum values and names match tommy's
// pkg/cst.NodeKind so that migration code can map between them directly.
type NodeKind int

const (
	NodeDocument     NodeKind = iota // Root of the document
	NodeTable                        // [table]
	NodeArrayTable                   // [[array-of-tables]]
	NodeKeyValue                     // key = value
	NodeKey                          // bare or quoted key
	NodeEquals                       // =
	NodeString                       // "...", '...', """...""", '''...'''
	NodeInteger                      // 42, 0xDEAD, 0o755, 0b1101
	NodeFloat                        // 3.14, inf, nan
	NodeBool                         // true, false
	NodeDateTime                     // 1979-05-27T07:32:00Z
	NodeArray                        // [1, 2, 3]
	NodeInlineTable                  // {a = 1, b = 2}
	NodeComment                      // # ...
	NodeWhitespace                   // spaces and tabs
	NodeNewline                      // \n or \r\n
	NodeBracketOpen                  // [
	NodeBracketClose                 // ]
	NodeBraceOpen                    // {
	NodeBraceClose                   // }
	NodeComma                        // ,
	NodeDot                          // .
)

// Node is a concrete syntax tree node. Leaf nodes have Raw bytes and no
// Children. Container nodes have Children and no Raw bytes. Bytes()
// recursively concatenates all descendant Raw bytes to reproduce the
// original source text.
type Node struct {
	Kind     NodeKind
	Raw      []byte
	Children []*Node
}

// Bytes returns the source text represented by this node. For leaf nodes,
// it returns Raw directly. For container nodes, it recursively concatenates
// all children's bytes.
func (n *Node) Bytes() []byte {
	if len(n.Children) == 0 {
		return n.Raw
	}
	var out []byte
	for _, child := range n.Children {
		out = append(out, child.Bytes()...)
	}
	return out
}
