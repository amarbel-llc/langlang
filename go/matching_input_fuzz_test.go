package langlang

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Matching-input fuzzer — the deepest step, and the true dual of tommy's
// algebraic fuzzer. tommy's TestRoundTripFuzz generates a random *type* and a
// *value inhabiting it*, then asserts decode(encode(value)) == value. The PEG
// analog is to generate a random *grammar* and a *string in its language*, then
// assert the matcher accepts it. The previous fuzzers only ever fed the matcher
// arbitrary inputs (most rejected); this one feeds it inputs that are
// constructed to be in the grammar, exercising the accept path end-to-end.
//
// Soundness — the hard part. A string is guaranteed in the language only if it
// follows PEG's deterministic matching. The generator (execSafe + noRepetition
// + noPredicate) restricts grammars to the star-free, predicate-free subset
// where every construct consumes a bounded, exactly-known amount, so a string
// co-generated from the grammar matches and fully consumes deterministically:
//   - atom (Literal/Class/Any): consumes exactly its one sample.
//   - sequence: the concatenation of its items' samples.
//   - choice: the FIRST branch's sample — PEG tries the first branch first and
//     commits when it matches, so first-branch strings always take that path.
//   - optional e?: emit e's sample (never empty). `?` is greedy, so it consumes
//     the present match; emitting empty would let `?` eat a following overlap.
//   - lex #e: e's sample (lex only toggles auto-spacing, which is moot with
//     space-free samples).
// Excluded (unsound to co-generate without first-set/context coordination):
//   - * / + : unbounded-greedy, eat into a following sibling on overlap
//     (`[a-z]* 'm'` rejects "am"); and & / ! : couple to the following context.
// Empirically this subset is exact — across thousands of generated grammars
// every co-generated string both matched and consumed the whole input — so the
// acceptance and full-consumption checks below are hard assertions.
//
// Determinism: shares the LANGLANG_FUZZ_* knobs with the other fuzzers.

// sampleString co-generates a string in the language of n, recursing through
// rule references (acyclic in execSafe mode, so this terminates).
func sampleString(n AstNode, defs map[string]*DefinitionNode) string {
	switch x := n.(type) {
	case *AnyNode:
		return "a" // any single byte matches "."
	case *LiteralNode:
		return x.Value
	case *ClassNode:
		// The left endpoint of the first range is guaranteed inside the class.
		if len(x.Items) > 0 {
			if rg, ok := x.Items[0].(*RangeNode); ok {
				return string(rg.Left)
			}
		}
		return "a"
	case *IdentifierNode:
		return sampleString(defs[x.Value].Expr, defs)
	case *OptionalNode:
		return sampleString(x.Expr, defs)
	case *LexNode:
		return sampleString(x.Expr, defs)
	case *SequenceNode:
		var b strings.Builder
		for _, it := range x.Items {
			b.WriteString(sampleString(it, defs))
		}
		return b.String()
	case *ChoiceNode:
		return sampleString(x.Left, defs)
	default:
		// * / + / & / ! are excluded from the co-generatable subset.
		panic(fmt.Sprintf("sampleString: unexpected node %T in co-generatable grammar", n))
	}
}

func TestMatchingInputFuzz(t *testing.T) {
	seed := fuzzEnvInt("LANGLANG_FUZZ_SEED", 1)
	cases := fuzzEnvInt("LANGLANG_FUZZ_CASES", 200)
	maxDefs := fuzzEnvInt("LANGLANG_FUZZ_MAXDEFS", 4)
	depth := fuzzEnvInt("LANGLANG_FUZZ_DEPTH", 4)

	rng := rand.New(rand.NewSource(int64(seed)))
	matched := 0
	for c := 0; c < cases; c++ {
		t.Run(fmt.Sprintf("case-%d", c), func(t *testing.T) {
			g := &shapeGen{
				rng:          rng,
				numDefs:      1 + rng.Intn(maxDefs),
				execSafe:     true,
				noRepetition: true,
				noPredicate:  true,
			}
			grammar := g.genGrammar(depth)
			src := grammar.String()

			m, err := MatcherFromString(src)
			if err != nil {
				t.Logf("grammar did not compile (skipping) for:\n%s\nerr: %v", src, err)
				return
			}

			input := []byte(sampleString(grammar.Definitions[0].Expr, grammar.DefsByName))
			res, done := runMatch(m, input, 2*time.Second)
			require.Truef(t, done, "Match did not terminate\ngrammar:\n%s\ninput: %q", src, input)
			require.Nilf(t, res.panicked, "Match panicked: %v\ngrammar:\n%s\ninput: %q", res.panicked, src, input)

			require.NoErrorf(t, res.err, "co-generated in-language string was REJECTED\ngrammar:\n%s\ninput: %q", src, input)
			require.Equalf(t, len(input), res.consumed, "in-language string matched but did not fully consume (%d of %d bytes)\ngrammar:\n%s\ninput: %q", res.consumed, len(input), src, input)
			checkTree(t, res, src, input)
			matched++
		})
	}
	require.Positivef(t, matched, "no generated grammar produced a matched in-language string across %d cases", cases)
}
