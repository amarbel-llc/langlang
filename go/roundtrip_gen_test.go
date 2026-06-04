package langlang

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// Generative round-trip property test, adapted from tommy's algebraic fuzzer
// (generate/roundtrip_gen_test.go). Where tommy generates random *legal* nested
// type shapes over its Scalar/Ptr/Slice/Map/Struct type algebra and asserts
// decode(encode(v)) == v, this generates random *legal* grammar ASTs over
// langlang's PEG expression algebra (grammar_ast.go) and asserts
// parse(g.String()) == g — i.e. the AST printer and the (bootstrapped) grammar
// parser round-trip. It catches printer/parser drift in CI rather than in a
// downstream migration.
//
// Scope: the structural core of the algebra — Choice / Sequence, the prefix
// operators (! & #), the suffix operators (? * +), and the atoms (rule
// reference, Literal, Any, and a character Class of ranges). Generation is
// indexed by PEG operator precedence (Choice > Sequence > Prefix > Suffix >
// Primary) so that String() is unambiguous. The AST→text printer emits no
// parentheses, so any shape that would *need* grouping to round-trip is
// deliberately not produced — the analog of tommy's fuzzer excluding the
// text/custom codecs and cross-package delegation, which live elsewhere.
//
// Out of scope for the same printer/round-trip reasons (left for a follow-up,
// e.g. a compile→VM property or a grouping-aware printer): parenthesized
// grouping, Labeled/Capture/Precedence, NameBinding/BytesConsume/
// CountedRepetition, NumericPrimitive, Import, and the post-parse Charset form.
//
// Determinism: a fixed seed (override LANGLANG_FUZZ_SEED) and case count
// (LANGLANG_FUZZ_CASES) so CI runs the same shapes every time and failures
// reproduce. On failure the generated grammar source is printed.

// fuzzLoc is the source location stamped on every generated node. AstNode.Equal
// ignores source positions, so a zero value is fine — only structure matters.
var fuzzLoc = SourceLocation{}

// shapeGen carries the per-grammar generation state: the seeded RNG and the
// number of rules in scope (R0..R{numDefs-1}), so a primary can emit a
// reference to any defined rule.
type shapeGen struct {
	rng     *rand.Rand
	numDefs int
}

func (g *shapeGen) ruleName(i int) string { return "R" + strconv.Itoa(i) }

// literalText builds a single-quoted-literal body restricted to characters that
// survive LiteralNode.String() (which quotes raw, without escaping): lowercase
// letters and digits, never a quote or backslash.
func (g *shapeGen) literalText() string {
	const alpha = "abcdefghijklmnopqrstuvwxyz0123456789"
	n := 1 + g.rng.Intn(3) // 1..3 chars
	b := make([]byte, n)
	for i := range b {
		b[i] = alpha[g.rng.Intn(len(alpha))]
	}
	return string(b)
}

// genClass builds a [..] character class of 1..3 ranges drawn from a single
// alphabet block, so RangeNode.String() ("a-z") reparses cleanly. We emit only
// ranges (never bare single chars): a single char inside a class round-trips to
// a LiteralNode, whose String() re-quotes it ('x') and breaks the class form.
func (g *shapeGen) genClass() AstNode {
	blocks := [][2]rune{{'a', 'z'}, {'A', 'Z'}, {'0', '9'}}
	n := 1 + g.rng.Intn(3)
	items := make([]AstNode, n)
	for i := range items {
		blk := blocks[g.rng.Intn(len(blocks))]
		span := int(blk[1] - blk[0])
		lo := blk[0] + rune(g.rng.Intn(span+1))
		hi := lo + rune(g.rng.Intn(int(blk[1]-lo)+1))
		items[i] = NewRangeNode(lo, hi, fuzzLoc)
	}
	return NewClassNode(items, fuzzLoc)
}

// genPrimary emits an atom: a rule reference, a literal, any (.), or a class.
// Atoms never recurse into a sub-expression — without printable parentheses a
// grouped expression cannot round-trip — so the algebra here is intentionally
// flat below the suffix level.
func (g *shapeGen) genPrimary() AstNode {
	switch g.rng.Intn(4) {
	case 0:
		return NewAnyNode(fuzzLoc)
	case 1:
		return NewLiteralNode(g.literalText(), fuzzLoc)
	case 2:
		return g.genClass()
	default:
		return NewIdentifierNode(g.ruleName(g.rng.Intn(g.numDefs)), fuzzLoc)
	}
}

// genSuffixed optionally wraps a primary in one suffix operator. The parser
// allows a single suffix on a primary (Primary ("?"/"*"/"+")?), so we never
// stack them.
func (g *shapeGen) genSuffixed(depth int) AstNode {
	prim := g.genPrimary()
	if depth <= 0 || g.rng.Intn(2) == 1 {
		return prim
	}
	switch g.rng.Intn(3) {
	case 0:
		return NewOptionalNode(prim, fuzzLoc)
	case 1:
		return NewZeroOrMoreNode(prim, fuzzLoc)
	default:
		return NewOneOrMoreNode(prim, fuzzLoc)
	}
}

// genItem produces one sequence element: a suffixed primary, optionally with a
// prefix operator (! & #) applied. A prefix binds tighter than sequence, so
// "!a b" is (!a) b — exactly how genItem nests it.
func (g *shapeGen) genItem(depth int) AstNode {
	inner := g.genSuffixed(depth)
	if depth <= 0 || g.rng.Intn(3) != 0 {
		return inner
	}
	switch g.rng.Intn(3) {
	case 0:
		return NewNotNode(inner, fuzzLoc)
	case 1:
		return NewAndNode(inner, fuzzLoc)
	default:
		return NewLexNode(inner, fuzzLoc)
	}
}

// genSequence builds a 1..3 element SequenceNode. The parser always wraps a
// choice branch in a Sequence (even a single element), so we mirror that shape
// for structural equality.
func (g *shapeGen) genSequence(depth int) AstNode {
	n := 1
	if depth > 0 {
		n = 1 + g.rng.Intn(3)
	}
	items := make([]AstNode, n)
	for i := range items {
		items[i] = g.genItem(depth - 1)
	}
	return NewSequenceNode(items, fuzzLoc)
}

// genExpression builds 1..3 sequence alternatives folded into a
// right-associative Choice, matching parseChoice's fold (a / b / c parses as
// Choice(a, Choice(b, c))).
func (g *shapeGen) genExpression(depth int) AstNode {
	alts := 1
	if depth > 0 {
		alts = 1 + g.rng.Intn(3)
	}
	accum := g.genSequence(depth - 1)
	branches := make([]AstNode, alts-1)
	for i := range branches {
		branches[i] = g.genSequence(depth - 1)
	}
	for i := len(branches) - 1; i >= 0; i-- {
		accum = NewChoiceNode(branches[i], accum, fuzzLoc)
	}
	return accum
}

// genGrammar emits numDefs definitions named R0..R{numDefs-1}.
func (g *shapeGen) genGrammar(depth int) *GrammarNode {
	defs := make([]*DefinitionNode, g.numDefs)
	byName := map[string]*DefinitionNode{}
	for i := range defs {
		name := g.ruleName(i)
		def := NewDefinitionNode(name, g.genExpression(depth), fuzzLoc)
		defs[i] = def
		byName[name] = def
	}
	return NewGrammarNode(nil, defs, byName, fuzzLoc)
}

func fuzzEnvInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func TestRoundTripFuzz(t *testing.T) {
	seed := fuzzEnvInt("LANGLANG_FUZZ_SEED", 1)
	cases := fuzzEnvInt("LANGLANG_FUZZ_CASES", 200)
	maxDefs := fuzzEnvInt("LANGLANG_FUZZ_MAXDEFS", 4)
	depth := fuzzEnvInt("LANGLANG_FUZZ_DEPTH", 4)

	rng := rand.New(rand.NewSource(int64(seed)))
	for c := 0; c < cases; c++ {
		t.Run(fmt.Sprintf("case-%d", c), func(t *testing.T) {
			g := &shapeGen{rng: rng, numDefs: 1 + rng.Intn(maxDefs)}
			want := g.genGrammar(depth)
			src := want.String()

			got, err := NewGrammarParser([]byte(src)).Parse()
			require.NoErrorf(t, err, "parse failed for:\n%s", src)

			gn, ok := got.(*GrammarNode)
			require.Truef(t, ok, "parse returned %T, not *GrammarNode, for:\n%s", got, src)
			require.Emptyf(t, gn.Errors, "parse produced errors for:\n%s\nerrors: %v", src, gn.Errors)

			if !want.Equal(gn) {
				t.Fatalf("round-trip structural mismatch\n--- source ---\n%s\n--- reparsed ---\n%s", src, gn.String())
			}
			// Print idempotence: re-printing the reparsed AST must be byte-identical.
			if got2 := gn.String(); got2 != src {
				t.Fatalf("print not idempotent\n--- first ---\n%s\n--- second ---\n%s", src, got2)
			}
		})
	}
}
