package junction

import (
	"testing"

	langlang "github.com/clarete/langlang/go"
)

// loc is a zero-value source location for synthetic AST nodes.
var loc langlang.SourceLocation

// TestIndirectDelimiterResolution verifies that the analyzer correctly
// classifies open/close brackets when delimiters are referenced through
// named rules (e.g., LBRACE <- '{' Skip) rather than appearing as
// direct literals in the bracket sequence.
//
// This is the pattern used by Go, C-family, and other grammars that
// abstract token rules.
func TestIndirectDelimiterResolution(t *testing.T) {
	// Synthetic grammar (mimics what the PEG pipeline produces):
	//
	//   Spacing <- ' '*
	//   Skip    <- ' '*
	//   LBRACE  <- Spacing [{] Spacing Skip    (pipeline transforms '{' to [{])
	//   RBRACE  <- Spacing [}] Spacing Spacing
	//   Item    <- [a-z]+
	//   Items   <- (Item)*
	//   Block   <- Spacing LBRACE Spacing Items? Spacing RBRACE

	spacing := langlang.NewDefinitionNode("Spacing",
		langlang.NewZeroOrMoreNode(langlang.NewLiteralNode(" ", loc), loc), loc)

	skip := langlang.NewDefinitionNode("Skip",
		langlang.NewZeroOrMoreNode(langlang.NewLiteralNode(" ", loc), loc), loc)

	// LBRACE after pipeline: [Spacing, [{], Spacing, Skip]
	lbrace := langlang.NewDefinitionNode("LBRACE",
		langlang.NewCaptureNode("LBRACE",
			langlang.NewSequenceNode([]langlang.AstNode{
				langlang.NewIdentifierNode("Spacing", loc),
				langlang.NewLiteralNode("{", loc),
				langlang.NewIdentifierNode("Spacing", loc),
				langlang.NewIdentifierNode("Skip", loc),
			}, loc), loc), loc)

	// RBRACE after pipeline: [Spacing, [}], Spacing, Spacing]
	rbrace := langlang.NewDefinitionNode("RBRACE",
		langlang.NewCaptureNode("RBRACE",
			langlang.NewSequenceNode([]langlang.AstNode{
				langlang.NewIdentifierNode("Spacing", loc),
				langlang.NewLiteralNode("}", loc),
				langlang.NewIdentifierNode("Spacing", loc),
				langlang.NewIdentifierNode("Spacing", loc),
			}, loc), loc), loc)

	item := langlang.NewDefinitionNode("Item",
		langlang.NewOneOrMoreNode(langlang.NewAnyNode(loc), loc), loc)

	items := langlang.NewDefinitionNode("Items",
		langlang.NewZeroOrMoreNode(
			langlang.NewIdentifierNode("Item", loc), loc), loc)

	// Block after pipeline: [Spacing, LBRACE, Spacing, Items?, Spacing, RBRACE]
	block := langlang.NewDefinitionNode("Block",
		langlang.NewCaptureNode("Block",
			langlang.NewSequenceNode([]langlang.AstNode{
				langlang.NewIdentifierNode("Spacing", loc),
				langlang.NewIdentifierNode("LBRACE", loc),
				langlang.NewIdentifierNode("Spacing", loc),
				langlang.NewOptionalNode(langlang.NewIdentifierNode("Items", loc), loc),
				langlang.NewIdentifierNode("Spacing", loc),
				langlang.NewIdentifierNode("RBRACE", loc),
			}, loc), loc), loc)

	defs := map[string]*langlang.DefinitionNode{
		"Spacing": spacing, "Skip": skip,
		"LBRACE": lbrace, "RBRACE": rbrace,
		"Item": item, "Items": items, "Block": block,
	}

	a := &analyzer{
		defs: defs,
		seen: make(map[byte]bool),
	}

	// Run pass 1 (brackets) then pass 2 (separators).
	allDefs := []*langlang.DefinitionNode{spacing, skip, lbrace, rbrace, item, items, block}
	for _, def := range allDefs {
		if isLexWrapped(def.Expr) {
			continue
		}
		expr := unwrapTransparent(def.Expr)
		if seq, ok := expr.(*langlang.SequenceNode); ok {
			a.analyzeSequenceForBrackets(seq)
		}
	}
	for _, def := range allDefs {
		if isLexWrapped(def.Expr) {
			continue
		}
		expr := unwrapTransparent(def.Expr)
		if seq, ok := expr.(*langlang.SequenceNode); ok {
			a.analyzeSequenceForSeparators(seq)
			a.analyzeSequenceForQuoting(seq)
		}
	}

	wantJunctions := map[byte]JunctionKind{
		'{': JunctionOpen,
		'}': JunctionClose,
	}

	assertJunctions(t, a.spec.Junctions, wantJunctions)
}
