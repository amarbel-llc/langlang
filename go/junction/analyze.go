package junction

import (
	"fmt"

	langlang "github.com/clarete/langlang/go"
)

// AnalyzeForJunctions walks a PEG grammar AST and derives a ScannerSpec
// describing which bytes are structural junctions and which delimiters
// define quoting contexts.
func AnalyzeForJunctions(grammarPath string) (ScannerSpec, error) {
	cfg := langlang.NewConfig()
	loader := langlang.NewRelativeImportLoader()
	db := langlang.NewDatabase(cfg, loader)

	grammar, err := langlang.QueryAST(db, grammarPath)
	if err != nil {
		return ScannerSpec{}, fmt.Errorf("query AST: %w", err)
	}

	a := &analyzer{
		defs: grammar.DefsByName,
		seen: make(map[byte]bool),
	}

	for _, def := range grammar.Definitions {
		a.analyzeDefinition(def)
	}

	return a.spec, nil
}

type analyzer struct {
	defs map[string]*langlang.DefinitionNode
	spec ScannerSpec
	seen map[byte]bool
}

func (a *analyzer) analyzeDefinition(def *langlang.DefinitionNode) {
	// Skip definitions whose body is lex-wrapped (#(...)) — these are
	// lexical rules and should not produce structural junctions.
	if isLexWrapped(def.Expr) {
		return
	}

	expr := unwrapTransparent(def.Expr)

	switch e := expr.(type) {
	case *langlang.SequenceNode:
		a.analyzeSequenceForJunctions(e)
		a.analyzeSequenceForQuoting(e)
	}
}

// isLexWrapped returns true if the node is a LexNode or is wrapped in
// transparent nodes (Label, Capture) around a LexNode.
func isLexWrapped(n langlang.AstNode) bool {
	for {
		switch e := n.(type) {
		case *langlang.LexNode:
			return true
		case *langlang.LabeledNode:
			n = e.Expr
		case *langlang.CaptureNode:
			n = e.Expr
		default:
			return false
		}
	}
}

// analyzeSequenceForJunctions classifies literal children of a sequence
// as Open, Close, or Separator junctions based on their position relative
// to repetition nodes. Only sequences with at least two single-byte
// literals bracketing a repetition are considered (to avoid treating
// prefix literals like '.' in Frac as junctions).
func (a *analyzer) analyzeSequenceForJunctions(seq *langlang.SequenceNode) {
	if !a.sequenceHasRepetition(seq) {
		return
	}

	// Count single-byte literals in this sequence.
	var literalIndices []int
	for i, item := range seq.Items {
		if _, ok := singleByteFromNode(unwrapTransparent(item)); ok {
			literalIndices = append(literalIndices, i)
		}
	}

	// Need at least 2 literals to form an open/close pair.
	if len(literalIndices) < 2 {
		// Even without open/close, look for separators inside repetitions.
		for _, item := range seq.Items {
			a.findSeparatorsInRepetition(unwrapTransparent(item))
		}
		return
	}

	firstIdx := literalIndices[0]
	lastIdx := literalIndices[len(literalIndices)-1]

	for _, i := range literalIndices {
		inner := unwrapTransparent(seq.Items[i])
		b, _ := singleByteFromNode(inner)

		if a.seen[b] {
			continue
		}

		var kind JunctionKind
		switch {
		case i == firstIdx:
			kind = JunctionOpen
		case i == lastIdx:
			kind = JunctionClose
		default:
			kind = JunctionSeparator
		}

		a.seen[b] = true
		a.spec.Junctions = append(a.spec.Junctions, JunctionByte{Byte: b, Kind: kind})
	}

	// Also look for separators inside repetition children.
	for _, item := range seq.Items {
		a.findSeparatorsInRepetition(unwrapTransparent(item))
	}
}

// findSeparatorsInRepetition recursively descends into repetition and
// optional nodes to find separator literals, including following rule
// references one level deep.
func (a *analyzer) findSeparatorsInRepetition(node langlang.AstNode) {
	a.findSeparatorsInRepetitionDepth(node, 0)
}

func (a *analyzer) findSeparatorsInRepetitionDepth(node langlang.AstNode, depth int) {
	if depth > 3 {
		return
	}
	switch e := node.(type) {
	case *langlang.ZeroOrMoreNode:
		a.findSeparatorsInRepetitionDepth(unwrapTransparent(e.Expr), depth)
	case *langlang.OneOrMoreNode:
		a.findSeparatorsInRepetitionDepth(unwrapTransparent(e.Expr), depth)
	case *langlang.OptionalNode:
		a.findSeparatorsInRepetitionDepth(unwrapTransparent(e.Expr), depth)
	case *langlang.SequenceNode:
		for _, item := range e.Items {
			inner := unwrapTransparent(item)
			if b, ok := singleByteFromNode(inner); ok {
				if !a.seen[b] {
					a.seen[b] = true
					a.spec.Junctions = append(a.spec.Junctions, JunctionByte{Byte: b, Kind: JunctionSeparator})
				}
				continue
			}
			a.findSeparatorsInRepetitionDepth(inner, depth)
		}
	case *langlang.IdentifierNode:
		// Follow rule references to find separators in referenced rules.
		if def, ok := a.defs[e.Value]; ok {
			a.findSeparatorsInReferencedRule(unwrapTransparent(def.Expr), depth+1)
		}
	}
}

// findSeparatorsInReferencedRule looks for single-byte literals in a
// referenced rule's sequence that act as separators.
func (a *analyzer) findSeparatorsInReferencedRule(node langlang.AstNode, depth int) {
	if depth > 3 {
		return
	}
	switch e := node.(type) {
	case *langlang.SequenceNode:
		for _, item := range e.Items {
			inner := unwrapTransparent(item)
			if b, ok := singleByteFromNode(inner); ok {
				if !a.seen[b] {
					a.seen[b] = true
					a.spec.Junctions = append(a.spec.Junctions, JunctionByte{Byte: b, Kind: JunctionSeparator})
				}
			}
		}
	}
}

// analyzeSequenceForQuoting detects quoting contexts: a sequence starting
// with a single-byte literal delimiter where the rest of the expression
// contains a repetition and the same delimiter byte appears again as a
// closing marker (possibly nested inside LexNode wrappers).
func (a *analyzer) analyzeSequenceForQuoting(seq *langlang.SequenceNode) {
	if len(seq.Items) < 2 {
		return
	}

	// Find the first single-byte literal in the sequence (skipping
	// injected Spacing identifiers and other non-literal nodes).
	var delim byte
	delimIdx := -1
	for i, item := range seq.Items {
		if b, ok := singleByteFromNode(unwrapTransparent(item)); ok {
			delim = b
			delimIdx = i
			break
		}
	}
	if delimIdx < 0 {
		return
	}

	hasRepetition := false
	hasClosingDelim := false
	for _, item := range seq.Items[delimIdx+1:] {
		if containsRepetition(unwrapTransparent(item)) {
			hasRepetition = true
		}
		if containsByte(unwrapTransparent(item), delim) {
			hasClosingDelim = true
		}
	}

	if !hasRepetition || !hasClosingDelim {
		return
	}

	qc := QuotingContext{Delimiter: delim}

	// Look for escape prefix by following rule references.
	for _, item := range seq.Items[1:] {
		qc.EscapePrefix = a.findEscapePrefix(unwrapTransparent(item), 0)
		if qc.EscapePrefix != 0 {
			break
		}
	}

	for _, existing := range a.spec.Quoting {
		if existing.Delimiter == qc.Delimiter {
			return
		}
	}
	a.spec.Quoting = append(a.spec.Quoting, qc)
}

// findEscapePrefix recursively searches for a literal escape prefix
// (e.g. '\\') at the start of a choice or sequence, following rule
// references up to maxDepth to avoid infinite recursion.
func (a *analyzer) findEscapePrefix(node langlang.AstNode, depth int) byte {
	if depth > 5 {
		return 0
	}
	switch e := node.(type) {
	case *langlang.ZeroOrMoreNode:
		return a.findEscapePrefix(unwrapTransparent(e.Expr), depth)
	case *langlang.OneOrMoreNode:
		return a.findEscapePrefix(unwrapTransparent(e.Expr), depth)
	case *langlang.ChoiceNode:
		if b := a.findEscapePrefix(unwrapTransparent(e.Left), depth); b != 0 {
			return b
		}
		return a.findEscapePrefix(unwrapTransparent(e.Right), depth)
	case *langlang.SequenceNode:
		if len(e.Items) >= 2 {
			first := unwrapTransparent(e.Items[0])
			if b, ok := singleByteFromNode(first); ok && b == '\\' {
				return '\\'
			}
		}
		// Also recurse into sequence children to find escape patterns deeper.
		for _, item := range e.Items {
			if b := a.findEscapePrefix(unwrapTransparent(item), depth); b != 0 {
				return b
			}
		}
	case *langlang.IdentifierNode:
		if def, ok := a.defs[e.Value]; ok {
			return a.findEscapePrefix(unwrapTransparent(def.Expr), depth+1)
		}
	}
	return 0
}

// singleByteFromNode extracts a single byte from a LiteralNode or a
// single-character CharsetNode. Returns the byte and true if the node
// represents exactly one byte.
func singleByteFromNode(node langlang.AstNode) (byte, bool) {
	switch e := node.(type) {
	case *langlang.LiteralNode:
		if len(e.Value) == 1 {
			return e.Value[0], true
		}
	case *langlang.CharsetNode:
		// CharsetNode.String() returns "[X]" for single-char charsets.
		// The charset uses escapeLiteral for special chars:
		//   " -> \"    \ -> \\    \n -> \n    \r -> \r    \t -> \t
		s := e.String()
		if len(s) == 3 && s[0] == '[' && s[2] == ']' {
			return s[1], true
		}
		// Handle escaped single chars: [\X] where X is the escaped form.
		if len(s) == 4 && s[0] == '[' && s[1] == '\\' && s[3] == ']' {
			switch s[2] {
			case '"':
				return '"', true
			case '\\':
				return '\\', true
			case 'n':
				return '\n', true
			case 'r':
				return '\r', true
			case 't':
				return '\t', true
			}
		}
	}
	return 0, false
}

func containsRepetition(node langlang.AstNode) bool {
	switch e := node.(type) {
	case *langlang.ZeroOrMoreNode, *langlang.OneOrMoreNode:
		return true
	case *langlang.SequenceNode:
		for _, item := range e.Items {
			if containsRepetition(unwrapTransparent(item)) {
				return true
			}
		}
	case *langlang.OptionalNode:
		return containsRepetition(unwrapTransparent(e.Expr))
	}
	return false
}

// containsByte checks whether a byte appears as a literal or single-char
// charset anywhere in the AST subtree.
func containsByte(node langlang.AstNode, target byte) bool {
	if b, ok := singleByteFromNode(node); ok {
		return b == target
	}
	switch e := node.(type) {
	case *langlang.SequenceNode:
		for _, item := range e.Items {
			if containsByte(unwrapTransparent(item), target) {
				return true
			}
		}
	case *langlang.ChoiceNode:
		return containsByte(unwrapTransparent(e.Left), target) ||
			containsByte(unwrapTransparent(e.Right), target)
	case *langlang.ZeroOrMoreNode:
		return containsByte(unwrapTransparent(e.Expr), target)
	case *langlang.OneOrMoreNode:
		return containsByte(unwrapTransparent(e.Expr), target)
	case *langlang.OptionalNode:
		return containsByte(unwrapTransparent(e.Expr), target)
	}
	return false
}

func (a *analyzer) sequenceHasRepetition(seq *langlang.SequenceNode) bool {
	for _, item := range seq.Items {
		inner := unwrapTransparent(item)
		switch inner.(type) {
		case *langlang.ZeroOrMoreNode, *langlang.OneOrMoreNode:
			return true
		case *langlang.OptionalNode:
			if a.optionalHasRepetition(inner.(*langlang.OptionalNode)) {
				return true
			}
		}
	}
	return false
}

func (a *analyzer) optionalHasRepetition(opt *langlang.OptionalNode) bool {
	inner := unwrapTransparent(opt.Expr)
	switch e := inner.(type) {
	case *langlang.SequenceNode:
		return a.sequenceHasRepetition(e)
	case *langlang.ZeroOrMoreNode, *langlang.OneOrMoreNode:
		return true
	case *langlang.IdentifierNode:
		if def, ok := a.defs[e.Value]; ok {
			return a.referencedRuleHasRepetition(unwrapTransparent(def.Expr))
		}
	}
	return false
}

func (a *analyzer) referencedRuleHasRepetition(node langlang.AstNode) bool {
	switch e := node.(type) {
	case *langlang.SequenceNode:
		return a.sequenceHasRepetition(e)
	case *langlang.ZeroOrMoreNode, *langlang.OneOrMoreNode:
		return true
	}
	return false
}

// unwrapTransparent strips AST wrappers that don't affect tree structure.
func unwrapTransparent(n langlang.AstNode) langlang.AstNode {
	for {
		switch e := n.(type) {
		case *langlang.LexNode:
			n = e.Expr
		case *langlang.LabeledNode:
			n = e.Expr
		case *langlang.CaptureNode:
			n = e.Expr
		default:
			return n
		}
	}
}
