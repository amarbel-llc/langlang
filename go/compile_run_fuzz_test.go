package langlang

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Generative compile-and-run robustness test — the second step adapted from
// tommy's algebraic fuzzer (see roundtrip_gen_test.go for the first). Where the
// round-trip fuzzer exercises the printer/parser, this drives the rest of the
// pipeline: it generates random *legal, execution-safe* grammars over the PEG
// expression algebra, compiles each to bytecode (MatcherFromString), and runs
// the VM over a handful of inputs. The property is robustness, not a specific
// parse result: the matcher must never panic, must always terminate, and on a
// successful match must return a well-formed tree (root present, node spans
// within input bounds, known node types). A clean parse error is an acceptable
// outcome.
//
// Safety: the VM has no step budget and does not guard nullable-body repetition
// (e* / e+ where e matches empty loops forever). The generator runs in
// execSafe mode, which only ever repeats a guaranteed-consuming terminal, so no
// generated grammar can construct that loop. Left recursion is already
// memoized-safe by the VM. As a belt-and-braces backstop each Match runs under
// a timeout and a panic recover, so a regression that reintroduces a hang or a
// crash fails the test (with a reproducer) instead of wedging CI.
//
// Determinism: shares the LANGLANG_FUZZ_* knobs with the round-trip fuzzer.

// matchResult captures everything a single Match invocation can produce,
// including a recovered panic.
type matchResult struct {
	tree     Tree
	consumed int
	err      error
	panicked any
}

// runMatch executes m.Match(input) on a goroutine, converting a panic into a
// result and bounding wall-clock time. The bool is false if the timeout fired
// (the matcher did not return in time).
func runMatch(m Matcher, input []byte, timeout time.Duration) (matchResult, bool) {
	ch := make(chan matchResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- matchResult{panicked: r}
			}
		}()
		tree, n, err := m.Match(input)
		ch <- matchResult{tree: tree, consumed: n, err: err}
	}()
	select {
	case res := <-ch:
		return res, true
	case <-time.After(timeout):
		return matchResult{}, false
	}
}

// checkTree asserts the structural invariants of a successful match: the
// consumed count is within bounds and, when a tree was produced, every node has
// a known type and a span contained in the input with Start <= End (Text() must
// not panic). A successful match may legitimately have no root — e.g. when the
// matching alternative is a lookahead predicate (&e / !e) or matches empty, the
// VM consumes nothing and captures nothing.
func checkTree(t *testing.T, res matchResult, src string, input []byte) {
	t.Helper()
	require.GreaterOrEqualf(t, res.consumed, 0, "negative consumed count %d\ngrammar:\n%s\ninput: %q", res.consumed, src, input)
	require.LessOrEqualf(t, res.consumed, len(input), "consumed %d > input len %d\ngrammar:\n%s\ninput: %q", res.consumed, len(input), src, input)

	root, ok := res.tree.Root()
	if !ok {
		return
	}
	res.tree.Visit(root, func(id NodeID) bool {
		ty := res.tree.Type(id)
		require.LessOrEqualf(t, ty, NodeType_Error, "unknown node type %d\ngrammar:\n%s\ninput: %q", ty, src, input)
		sp := res.tree.Span(id)
		require.GreaterOrEqualf(t, sp.Start.Cursor, 0, "node span start %d < 0\ngrammar:\n%s\ninput: %q", sp.Start.Cursor, src, input)
		require.LessOrEqualf(t, sp.End.Cursor, len(input), "node span end %d > input len %d\ngrammar:\n%s\ninput: %q", sp.End.Cursor, len(input), src, input)
		require.LessOrEqualf(t, sp.Start.Cursor, sp.End.Cursor, "node span start %d > end %d\ngrammar:\n%s\ninput: %q", sp.Start.Cursor, sp.End.Cursor, src, input)
		_ = res.tree.Text(id) // must not panic
		return true
	})
}

// collectTerminals gathers the literal strings and the left endpoint of each
// range used anywhere in the grammar, so inputsFor can bias inputs toward
// characters the grammar actually mentions. It only walks the node types the
// generator produces.
func collectTerminals(node AstNode) (lits []string, chars []rune) {
	var walk func(AstNode)
	walk = func(n AstNode) {
		switch x := n.(type) {
		case *GrammarNode:
			for _, d := range x.Definitions {
				walk(d)
			}
		case *DefinitionNode:
			walk(x.Expr)
		case *ChoiceNode:
			walk(x.Left)
			walk(x.Right)
		case *SequenceNode:
			for _, it := range x.Items {
				walk(it)
			}
		case *OptionalNode:
			walk(x.Expr)
		case *ZeroOrMoreNode:
			walk(x.Expr)
		case *OneOrMoreNode:
			walk(x.Expr)
		case *NotNode:
			walk(x.Expr)
		case *AndNode:
			walk(x.Expr)
		case *LexNode:
			walk(x.Expr)
		case *ClassNode:
			for _, it := range x.Items {
				walk(it)
			}
		case *RangeNode:
			chars = append(chars, x.Left)
		case *LiteralNode:
			lits = append(lits, x.Value)
		}
	}
	walk(node)
	return
}

// inputsFor builds n candidate inputs for a grammar: the empty string, one
// "designed" string concatenating a few of the grammar's own literals (to bias
// toward a match), and random alphanumeric strings (bounded length, to keep the
// unmemoized PEG runtime well clear of exponential blow-up) drawn from an
// alphabet that includes the grammar's class characters.
func (g *shapeGen) inputsFor(grammar *GrammarNode, n int) [][]byte {
	lits, chars := collectTerminals(grammar)
	alpha := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	for _, ch := range chars {
		if ch < 128 {
			alpha = append(alpha, byte(ch))
		}
	}

	out := [][]byte{[]byte("")}
	if len(lits) > 0 {
		var b strings.Builder
		for i := 0; i < 3; i++ {
			b.WriteString(lits[g.rng.Intn(len(lits))])
		}
		out = append(out, []byte(b.String()))
	}
	for len(out) < n {
		buf := make([]byte, g.rng.Intn(9)) // 0..8 bytes
		for i := range buf {
			buf[i] = alpha[g.rng.Intn(len(alpha))]
		}
		out = append(out, buf)
	}
	return out
}

func TestCompileRunFuzz(t *testing.T) {
	seed := fuzzEnvInt("LANGLANG_FUZZ_SEED", 1)
	cases := fuzzEnvInt("LANGLANG_FUZZ_CASES", 200)
	maxDefs := fuzzEnvInt("LANGLANG_FUZZ_MAXDEFS", 4)
	depth := fuzzEnvInt("LANGLANG_FUZZ_DEPTH", 4)

	rng := rand.New(rand.NewSource(int64(seed)))
	compiled := 0
	for c := 0; c < cases; c++ {
		t.Run(fmt.Sprintf("case-%d", c), func(t *testing.T) {
			g := &shapeGen{rng: rng, numDefs: 1 + rng.Intn(maxDefs), execSafe: true}
			grammar := g.genGrammar(depth)
			src := grammar.String()

			m, err := MatcherFromString(src)
			if err != nil {
				// A legal-to-parse grammar that fails to *compile* is allowed
				// (e.g. a semantic rejection); log it for visibility but do not
				// fail. The compiled-count assertion below guards against the
				// whole pipeline silently breaking.
				t.Logf("compile error (skipping run) for grammar:\n%s\nerr: %v", src, err)
				return
			}
			compiled++

			for _, input := range g.inputsFor(grammar, 6) {
				res, done := runMatch(m, input, 2*time.Second)
				require.Truef(t, done, "Match did not terminate within timeout\ngrammar:\n%s\ninput: %q", src, input)
				require.Nilf(t, res.panicked, "Match panicked: %v\ngrammar:\n%s\ninput: %q", res.panicked, src, input)
				if res.err == nil {
					checkTree(t, res, src, input)
				}
			}
		})
	}
	require.Positivef(t, compiled, "no generated grammar compiled across %d cases — generator or pipeline regression", cases)
}
