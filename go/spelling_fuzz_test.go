package langlang

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Spelling-equivalence fuzzer — the langlang adaptation of tommy's spelling
// fuzzer (#107, generate/roundtrip_spelling_test.go). tommy's observation: its
// round-trip fuzzers only ever feed the decoder ONE canonical spelling (the
// encoder's own output), so the other valid TOML spellings of the same value
// (inline tables vs sub-tables, dotted keys vs nested tables, ...) go untested;
// #107 re-spells the canonical encoding and checks each spelling decodes the
// same.
//
// The PEG parallel is exact, and it closes the gap the round-trip fuzzer
// (roundtrip_gen_test.go) left open: the AST printer (String()) emits no
// parentheses, so grouping cannot be tested via a *syntactic* round-trip. A PEG
// expression has multiple equivalent *surface spellings* — `a b c` and
// `(a b) c`, `a/b/c` and `(a/b)/c`, `e+` and `e e*` — that must compile and
// parse identically. So this fuzzer generates a canonical grammar, re-spells it,
// and asserts the spellings agree behaviourally (accept + bytes consumed) on a
// set of inputs. That is a *semantic*-equivalence property, testing exactly what
// the round-trip fuzzer could not.
//
// Classification mirrors tommy's, with hardness per variant:
//   - parenthesized (HARD): grouping is semantically transparent in langlang's
//     parser (a parenthesized expression is just a primary), so a disagreement
//     is a real bug and fails CI.
//   - plus-desugared (SOFT): `e+` == `e e*` holds for the recognized language,
//     but the rewrite is applied to the AST and rendered through the canonical
//     String() printer, which emits no parentheses. When the `+` sits under a
//     prefix/suffix operator (e.g. `&.+`), the desugared sequence loses its
//     grouping on the way to text (`&. .*` reparses as `(&.) .*`), so the
//     spelling diverges. That is logged as xfail (never fails CI) — the staged
//     "fuzzer-first" stance tommy takes for spellings not yet guaranteed — and
//     it re-demonstrates, from another angle, the printer grouping-loss that
//     motivates the parenthesized variant.
//
// A coverage guard requires that the rewrites actually changed the source (so
// the fuzzer is not silently comparing canonical against canonical).
//
// Reuses the execSafe shape generator and the compile/run helpers from
// roundtrip_gen_test.go and compile_run_fuzz_test.go, and shares the
// LANGLANG_FUZZ_* knobs.

// grammarSpelling is one semantics-preserving rewrite, paired with a serializer
// that renders the (re-spelled) grammar source from the canonical AST.
type grammarSpelling struct {
	name  string
	hard  bool // hard: a behavioural disagreement fails CI; soft: logged xfail
	spell func(*GrammarNode) string
}

var grammarSpellings = []grammarSpelling{
	{"parenthesized", true, spellParenthesized},
	{"plus-desugared", false, spellPlusDesugared},
}

// parenExpr renders an expression with every composite sub-expression wrapped in
// explicit parentheses. Atoms are bare; sequences, choices, and the operands of
// the prefix/suffix operators are grouped. Grouping is transparent to langlang's
// parser, so the result denotes the same language as the bare form.
func parenExpr(n AstNode) string {
	switch x := n.(type) {
	case *OptionalNode:
		return "(" + parenExpr(x.Expr) + ")?"
	case *ZeroOrMoreNode:
		return "(" + parenExpr(x.Expr) + ")*"
	case *OneOrMoreNode:
		return "(" + parenExpr(x.Expr) + ")+"
	case *NotNode:
		return "!(" + parenExpr(x.Expr) + ")"
	case *AndNode:
		return "&(" + parenExpr(x.Expr) + ")"
	case *LexNode:
		return "#(" + parenExpr(x.Expr) + ")"
	case *SequenceNode:
		parts := make([]string, len(x.Items))
		for i, it := range x.Items {
			parts[i] = parenExpr(it)
		}
		return "(" + strings.Join(parts, " ") + ")"
	case *ChoiceNode:
		return "(" + parenExpr(x.Left) + " / " + parenExpr(x.Right) + ")"
	default: // *AnyNode, *LiteralNode, *IdentifierNode, *ClassNode
		return n.String()
	}
}

func spellParenthesized(g *GrammarNode) string {
	var b strings.Builder
	for i, d := range g.Definitions {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(d.Name)
		b.WriteString(" <- ")
		b.WriteString(parenExpr(d.Expr))
	}
	return b.String()
}

// desugarPlus rewrites every `e+` to `e e*`, leaving the rest of the AST
// structurally identical. In execSafe mode `+` only wraps a (consuming)
// terminal, so duplicating the operand is side-effect-free.
func desugarPlus(n AstNode) AstNode {
	switch x := n.(type) {
	case *GrammarNode:
		defs := make([]*DefinitionNode, len(x.Definitions))
		byName := map[string]*DefinitionNode{}
		for i, d := range x.Definitions {
			nd := desugarPlus(d).(*DefinitionNode)
			defs[i] = nd
			byName[nd.Name] = nd
		}
		return NewGrammarNode(nil, defs, byName, fuzzLoc)
	case *DefinitionNode:
		return NewDefinitionNode(x.Name, desugarPlus(x.Expr), fuzzLoc)
	case *ChoiceNode:
		return NewChoiceNode(desugarPlus(x.Left), desugarPlus(x.Right), fuzzLoc)
	case *SequenceNode:
		items := make([]AstNode, len(x.Items))
		for i, it := range x.Items {
			items[i] = desugarPlus(it)
		}
		return NewSequenceNode(items, fuzzLoc)
	case *OptionalNode:
		return NewOptionalNode(desugarPlus(x.Expr), fuzzLoc)
	case *ZeroOrMoreNode:
		return NewZeroOrMoreNode(desugarPlus(x.Expr), fuzzLoc)
	case *OneOrMoreNode:
		e := desugarPlus(x.Expr)
		return NewSequenceNode([]AstNode{e, NewZeroOrMoreNode(e, fuzzLoc)}, fuzzLoc)
	case *NotNode:
		return NewNotNode(desugarPlus(x.Expr), fuzzLoc)
	case *AndNode:
		return NewAndNode(desugarPlus(x.Expr), fuzzLoc)
	case *LexNode:
		return NewLexNode(desugarPlus(x.Expr), fuzzLoc)
	default: // atoms
		return n
	}
}

func spellPlusDesugared(g *GrammarNode) string {
	return desugarPlus(g).(*GrammarNode).String()
}

// observation is the behaviour of a matcher on one input: whether it accepted
// and how many bytes it consumed. Two spellings are behaviourally equivalent on
// an input when their observations agree (consumed is only compared on accept).
type observation struct {
	ok       bool
	consumed int
}

func (o observation) eq(b observation) bool {
	return o.ok == b.ok && (!o.ok || o.consumed == b.consumed)
}

// observeAll runs m over every input under the timeout/panic backstop and
// returns one observation per input, failing the test on a hang or panic.
func observeAll(t *testing.T, m Matcher, inputs [][]byte, label, src string) []observation {
	t.Helper()
	obs := make([]observation, len(inputs))
	for i, in := range inputs {
		res, done := runMatch(m, in, 2*time.Second)
		require.Truef(t, done, "%s: Match did not terminate\ngrammar:\n%s\ninput: %q", label, src, in)
		require.Nilf(t, res.panicked, "%s: Match panicked: %v\ngrammar:\n%s\ninput: %q", label, res.panicked, src, in)
		obs[i] = observation{ok: res.err == nil, consumed: res.consumed}
	}
	return obs
}

func TestSpellingEquivalenceFuzz(t *testing.T) {
	seed := fuzzEnvInt("LANGLANG_FUZZ_SEED", 1)
	cases := fuzzEnvInt("LANGLANG_FUZZ_CASES", 200)
	maxDefs := fuzzEnvInt("LANGLANG_FUZZ_MAXDEFS", 4)
	depth := fuzzEnvInt("LANGLANG_FUZZ_DEPTH", 4)

	rng := rand.New(rand.NewSource(int64(seed)))
	var changed, xpass, xfail int
	for c := 0; c < cases; c++ {
		t.Run(fmt.Sprintf("case-%d", c), func(t *testing.T) {
			g := &shapeGen{rng: rng, numDefs: 1 + rng.Intn(maxDefs), execSafe: true}
			grammar := g.genGrammar(depth)
			canonical := grammar.String()

			base, err := MatcherFromString(canonical)
			if err != nil {
				t.Logf("canonical did not compile (skipping) for:\n%s\nerr: %v", canonical, err)
				return
			}
			inputs := g.inputsFor(grammar, 6)
			ref := observeAll(t, base, inputs, "canonical", canonical)

			for _, sp := range grammarSpellings {
				src := sp.spell(grammar)
				if src == canonical {
					continue // no-op rewrite: nothing new to compare
				}
				changed++

				m, err := MatcherFromString(src)
				if err != nil {
					if sp.hard {
						t.Fatalf("hard spelling %q failed to compile though canonical did\ncanonical:\n%s\n%s:\n%s\nerr: %v", sp.name, canonical, sp.name, src, err)
					}
					xfail++
					t.Logf("%s: xfail — re-spelled grammar did not compile: %v\n%s", sp.name, err, src)
					continue
				}

				got := observeAll(t, m, inputs, sp.name, src)
				agree := true
				for i := range inputs {
					if !ref[i].eq(got[i]) {
						agree = false
						if sp.hard {
							t.Fatalf("hard spelling %q disagrees with canonical on input %q\ncanonical (ok=%v n=%d):\n%s\n%s (ok=%v n=%d):\n%s",
								sp.name, inputs[i], ref[i].ok, ref[i].consumed, canonical, sp.name, got[i].ok, got[i].consumed, src)
						}
					}
				}
				if !sp.hard {
					if agree {
						xpass++
					} else {
						xfail++
						t.Logf("%s: xfail — alternative spelling diverges from canonical\ncanonical:\n%s\n%s:\n%s", sp.name, canonical, sp.name, src)
					}
				}
			}
		})
	}
	t.Logf("spelling fuzz: %d re-spellings exercised (soft variants: %d xpass, %d xfail)", changed, xpass, xfail)
	require.Positivef(t, changed, "no spelling rewrote the canonical grammar across %d cases — the fuzzer compared nothing", cases)
}
