# Bidirectional PEG Parsing via Grammar Reversal

## Concept

A PEG grammar is a bidirectional specification of structure. Given a grammar
that describes valid input forward, its mechanical reversal describes the same
structure backward. Two standard PEG parsers — one running the original grammar
forward from byte 0, the other running the reversed grammar backward from the
last byte — can parse the same input concurrently, converging toward the middle.

This is not speculative anchor scanning or heuristic partitioning. Both parsers
are doing real PEG parsing with full backtracking, ordered choice, and
repetition semantics. The grammar itself is the information source for both
directions.

## Grammar Reversal

Every PEG expression has a mechanical reversal. The transformation is applied
at codegen time to the grammar AST (`GrammarNode.Definitions`), producing a
second grammar that is fed through the standard compilation pipeline
(`QueryBytecode` → `GenGoEval`).

### Reversal Rules

| Forward construct              | Reversed construct                   | Notes                                                  |
|--------------------------------|--------------------------------------|--------------------------------------------------------|
| Sequence `A B C`              | `C_rev B_rev A_rev`                 | Order reversed, each element reversed recursively      |
| Literal `'abc'`               | `'cba'`                             | Byte order reversed                                    |
| Character class `[a-z]`       | `[a-z]`                             | Single byte, unchanged                                 |
| Range `[a-zA-Z0-9]`          | `[a-zA-Z0-9]`                       | Single byte, unchanged                                 |
| Any `.`                       | `.`                                 | Single byte, unchanged                                 |
| Choice `A / B`                | `A_rev / B_rev`                     | Priority order preserved (PEG choice is semantic)      |
| Zero-or-more `A*`            | `A_rev*`                            | Produces items in reverse document order                |
| One-or-more `A+`             | `A_rev+`                            | Produces items in reverse document order                |
| Optional `A?`                 | `A_rev?`                            | Unchanged structurally                                  |
| And-predicate `&A`           | `&A_rev`                            | Look-ahead becomes look-behind in original orientation |
| Not-predicate `!A`           | `!A_rev`                            | Same as above                                          |
| Capture `#name{ A }`         | `#name{ A_rev }`                    | Name preserved, body reversed                          |
| Rule reference `Identifier`  | `Identifier_rev`                    | Points to the reversed rule definition                 |

### Implementation

The reversal is a recursive AST visitor. For each `DefinitionNode` in the
`GrammarNode`, produce a new `DefinitionNode` with the reversed expression:

```go
func reverseExpr(expr AstNode) AstNode {
    switch n := expr.(type) {
    case *SequenceNode:
        reversed := make([]AstNode, len(n.Items))
        for i, item := range n.Items {
            reversed[len(n.Items)-1-i] = reverseExpr(item)
        }
        return NewSequenceNode(reversed, n.SourceLocation())

    case *LiteralNode:
        return NewLiteralNode(reverseString(n.Value), n.SourceLocation())

    case *ChoiceNode:
        return NewChoiceNode(reverseExpr(n.Left), reverseExpr(n.Right), n.SourceLocation())

    case *ZeroOrMoreNode:
        return NewZeroOrMoreNode(reverseExpr(n.Expr), n.SourceLocation())

    case *OneOrMoreNode:
        return NewOneOrMoreNode(reverseExpr(n.Expr), n.SourceLocation())

    case *OptionalNode:
        return NewOptionalNode(reverseExpr(n.Expr), n.SourceLocation())

    case *NotNode:
        return NewNotNode(reverseExpr(n.Expr), n.SourceLocation())

    case *AndNode:
        return NewAndNode(reverseExpr(n.Expr), n.SourceLocation())

    case *CaptureNode:
        return NewCaptureNode(n.Name, reverseExpr(n.Expr), n.SourceLocation())

    case *IdentifierNode:
        return NewIdentifierNode(n.Name+"_rev", n.SourceLocation())

    case *CharsetNode, *AnyNode, *RangeNode, *ClassNode:
        return expr // single-byte constructs are self-symmetric

    case *LexNode:
        return NewLexNode(reverseExpr(n.Expr), n.SourceLocation())

    case *LabeledNode:
        return NewLabeledNode(n.Label, reverseExpr(n.Expr), n.SourceLocation())

    default:
        panic(fmt.Sprintf("unhandled node type %T in grammar reversal", expr))
    }
}
```

The reversed grammar is a standalone grammar. It goes through the same
compilation pipeline as the forward grammar: parsing → AST → compiler →
bytecode → `GenGoEval`. The output is a second parser with its own bytecode
program, string table, and VM code.

## Execution Model

```
input bytes: [  0  1  2  3  4  5  6  7  8  9  ]

forward:      →→→→→→→→→→→→→→ consumed: 6
backward:                  ←←←←←←←←←← consumed: 4

forward parsed: rules A, B (bytes 0-5)
backward parsed: rules D, C (bytes 6-9, reversed)
```

Two goroutines, fully independent, no shared state:

```go
func parseBidirectional(input []byte) (result, error) {
    var fwd, bwd halfResult
    var wg sync.WaitGroup

    wg.Add(2)
    go func() {
        defer wg.Done()
        fwd = parseForward(input)
    }()
    go func() {
        defer wg.Done()
        bwd = parseBackward(input)  // direction handled internally
    }()
    wg.Wait()

    return merge(input, fwd, bwd)
}
```

Each parser has its own VM instance and arena. No locks, no channels beyond the
WaitGroup.

### halfResult

```go
type halfResult struct {
    consumed  int           // bytes consumed from this parser's starting end
    completed []ruleMatch   // which top-level rules were fully parsed
    partial   *ruleMatch    // rule in progress when the parser stopped (if any)
    err       error
}

type ruleMatch struct {
    ruleID    int    // nameID from the bytecode string table
    start     int    // byte offset in original input (not reversed)
    end       int    // byte offset in original input
    extracted any    // typed extraction result
}
```

The backward parser translates its cursor positions back to original-input
offsets before returning. This is a subtraction:
`originalPos = len(input) - 1 - reversedPos`.

## Input Access Without Copying

The naive approach copies the input into a reversed byte slice for the backward
parser. This costs `O(n)` allocation and `O(n)` copy time.

The alternative: abstract byte access in the VM so the backward parser reads the
original input in reverse without copying. The VM accesses input bytes in a
small number of code sites:

- Literal matching (compare cursor bytes against a literal string)
- Character class / range checking (test cursor byte against a bitset)
- Any-byte matching (advance cursor by one)
- Capture span recording (`start`, `end` offsets)

The abstraction is a cursor translator injected at VM construction:

```go
type cursorTranslator struct {
    input     []byte
    length    int
    direction int  // +1 for forward, -1 for backward
}

func (ct *cursorTranslator) byteAt(cursor int) byte {
    if ct.direction == 1 {
        return ct.input[cursor]
    }
    return ct.input[ct.length-1-cursor]
}
```

For the forward parser, `direction == 1` and `byteAt` is identity. The compiler
inlines this. For the backward parser, `direction == -1` and `byteAt` is a
subtraction. Branch prediction locks onto one direction since it never changes
within a parse.

The generated code (via `GenGoEval`) already copies the VM source into the
consumer package. The codegen can emit two VM variants — `forwardVM` and
`backwardVM` — where `byteAt` is a compile-time constant direction rather than
a runtime flag. This eliminates the branch entirely: each variant has the
direction baked into the code.

### Literal matching in reverse

The reversed grammar already has reversed literals (`'abc'` → `'cba'`). The
backward VM compares reversed literals against reversed-cursor bytes. Since both
are reversed, the comparison is byte-for-byte identical to the forward case.
No special handling needed.

### Capture span translation

The backward VM records `(start, end)` in reversed coordinates. Before
returning to the merge step, translate:

```go
originalStart = len(input) - reversedEnd
originalEnd   = len(input) - reversedStart
```

This applies to the `start` and `end` fields of every node in the backward
parser's arena. The translation is applied once, in a linear pass over the
arena, after parsing completes.

## Convergence and Merge

### Top-level sequences

For `Document <- A B C D`, the forward parser completes rules left to right
(A, B, C, D) and the backward parser completes rules right to left (D, C, B,
A). The merge matches completed rules by their position in the sequence:

```go
func mergeSequence(fwd, bwd halfResult, ruleOrder []int) (Document, error) {
    var doc Document
    fwdIdx := 0
    bwdIdx := len(bwd.completed) - 1  // backward results are in reverse order

    for _, ruleID := range ruleOrder {
        if fwdIdx < len(fwd.completed) && fwd.completed[fwdIdx].ruleID == ruleID {
            assignRule(&doc, ruleID, fwd.completed[fwdIdx])
            fwdIdx++
        } else if bwdIdx >= 0 && bwd.completed[bwdIdx].ruleID == ruleID {
            assignRule(&doc, ruleID, bwd.completed[bwdIdx])
            bwdIdx--
        } else {
            return doc, fmt.Errorf("rule %d not completed by either parser", ruleID)
        }
    }
    return doc, nil
}
```

### Possible merge outcomes

**Clean split:** `fwd.consumed + bwd.consumed == len(input)`. The parsers
covered the entire input with no overlap and no gap. Merge is concatenation.

**Overlap:** `fwd.consumed + bwd.consumed > len(input)`. Both parsers parsed
the same middle region. The overlapping rule must have produced the same result
from both directions. Verify by comparing the byte ranges. Discard one copy.

**Gap:** `fwd.consumed + bwd.consumed < len(input)`. Neither parser reached the
middle. The unparsed region `input[fwd.consumed : len(input)-bwd.consumed]` must
be parsed sequentially, starting from the forward parser's last cursor position.
This is the fallback path.

### Top-level repetitions

For `Document <- Block*`, both parsers produce partial lists. Forward produces
the first K blocks; backward produces the last M blocks. If `K + M` covers
all blocks, merge is concatenation with the backward list reversed. If there's
a gap, the middle region is parsed sequentially to fill it.

The gap detection for repetitions uses byte positions: if the forward parser's
last block ends at byte F and the backward parser's last block (in original
coordinates) starts at byte B, then `F < B` means there's a gap of
`input[F:B]` to fill.

## Stopping Condition

Neither parser knows when the other has "caught up." Two options:

### Option 1: Both parse to completion

Each parser tries to parse the entire input from its end. For many grammars,
the forward parser will complete first (it starts at byte 0, which is where
the grammar's start rule begins). The backward parser will also complete,
having parsed the same input in reverse. The merge discards the overlap.

The cost: both parsers do full work. Total CPU is 2x a single parse, but wall
time is ~1x (on 2 cores). The benefit is pure latency reduction, not throughput
improvement.

### Option 2: Cancellation on convergence

Each parser periodically publishes its cursor position (e.g., via an atomic
integer). When the sum of published positions exceeds `len(input)`, both
parsers have covered the full input. Send a cancellation signal.

```go
var fwdPos, bwdPos atomic.Int64

// In each parser's hot loop, periodically:
fwdPos.Store(int64(cursor))
if fwdPos.Load()+bwdPos.Load() >= int64(len(input)) {
    return earlyResult()
}
```

"Periodically" means once per top-level rule completion, not once per VM
instruction. The atomic load is cheap (~1 ns on x86) but should not be in the
inner loop.

The parser that receives the cancellation signal returns its partial result.
The merge handles whatever combination of complete/partial results it gets.

Option 2 is better for throughput (avoids redundant work) but adds complexity
to the VM's outer loop and to the merge logic. Option 1 is simpler and still
achieves the latency goal.

## Reversibility Constraints

Not all grammars are safely reversible. The codegen must validate before
emitting a backward parser.

### Safe

- Sequences of rules with fixed structure
- Literals and character classes (single-byte or multi-byte)
- Choices where all alternatives are independently reversible
- Repetitions of reversible rules
- Optional wrappers around reversible rules
- Predicates (`!` / `&`) applied to reversible expressions

### Requires analysis

- **Multi-byte look-ahead predicates.** `&('abc')` reversed to `&('cba')`
  is correct mechanically. But if the predicate appears inside a repetition
  and checks bytes that cross the boundary between the repeated element and
  its successor, the reversed version may check different relative bytes.
  This needs case-by-case analysis.

- **Left-recursive rules.** Langlang supports left recursion via memoization.
  The reversed grammar turns left recursion into right recursion (and vice
  versa). The memo table's convergence behavior may differ. The reversed
  grammar should be tested for parse equivalence before enabling bidirectional
  mode.

### Unsafe (codegen should refuse)

- **Stateful semantic actions.** Langlang's PEG is pure (no embedded code),
  so this doesn't arise in langlang itself. But any future extension that
  adds side effects to rules would break reversal.

- **Grammars where rule B's parse behavior depends on rule A's result.**
  Again, pure PEG doesn't have this, but parameterized rules or inherited
  attributes would break it.

### Validation strategy

The codegen walks the grammar AST and classifies each rule's reversibility.
Rules that are unconditionally safe get a reversed definition emitted. Rules
that require analysis are flagged — the backward parser treats them as
boundaries and stops before entering them, letting the forward parser handle
them. Rules that are unsafe cause the codegen to skip bidirectional mode
entirely for that grammar.

```go
type ReversibilityKind int

const (
    ReverseSafe        ReversibilityKind = iota  // mechanically reversible
    ReverseNeedsReview                           // may work, needs testing
    ReverseUnsafe                                // cannot reverse
)

func classifyReversibility(def *DefinitionNode) ReversibilityKind
```

## Repetition Order Correction

The backward parser produces items from repetitions (`A*`, `A+`) in reverse
document order. The extraction step must reverse them before producing the
final typed output.

For the typed arena strategies, this means the reader function for repeated
rules from the backward parser emits items into a slice and then reverses it:

```go
// Generated for backward parser:
func readBlocksBackward(arena []byte, input []byte, ref sliceRef) []Block {
    blocks := make([]Block, ref.count)
    for i := range blocks {
        off := int(ref.offset) + i*int(ref.stride)
        blocks[i] = readBlock(arena, input, off)
    }
    slices.Reverse(blocks)
    return blocks
}
```

This is a single `O(n)` pass after extraction. The cost is negligible relative
to parsing.

## Project Structure

```
go/
├── reverse/                       # grammar reversal
│   ├── reverse.go                 # AST reversal transformation
│   ├── reverse_test.go            # round-trip: reverse(reverse(grammar)) == grammar
│   ├── validate.go                # reversibility classification
│   └── validate_test.go
├── bidi/                          # bidirectional orchestration
│   ├── orchestrator.go            # goroutine management, merge logic
│   ├── orchestrator_test.go
│   ├── cursor.go                  # cursor translation (forward/backward)
│   └── cursor_test.go
├── gen.go                         # modified: emit forward + backward parsers
└── cmd/
    └── langlang/
        └── bidi.go                # CLI flag: --bidi for bidirectional mode
```

## Verification

### Correctness

The reversed grammar must produce equivalent parse results to the forward
grammar. For every input that the forward parser accepts, the backward parser
must accept `reverse(input)` and produce a tree that, after position
translation and repetition reversal, matches the forward tree.

Test strategy: property-based testing. Generate random valid inputs from the
grammar (langlang already has test infrastructure for this), parse forward,
parse backward, compare extracted results.

```go
func TestBidiEquivalence(t *testing.T) {
    for _, input := range generatedInputs {
        fwd := parseForward(input)
        bwd := parseBackward(input)
        translated := translatePositions(bwd, len(input))
        if !equivalent(fwd, translated) {
            t.Errorf("mismatch on input %q", input)
        }
    }
}
```

### Round-trip reversal

`reverse(reverse(grammar))` must produce a grammar identical to the original.
This is a structural invariant of the reversal transformation and should be
tested at the AST level.

## Benchmark Integration

This exploration is independent of the arena benchmark plan. When benchmarked,
it should be measured as an orthogonal axis:

```
                    │ sequential │ bidirectional │
────────────────────┼────────────┼───────────────┤
baseline (Level 0)  │     A      │       E       │
strategy B (typed)  │     B      │       F       │
```

Cells A and B come from the arena benchmark plan. Cells E and F measure the
same arena strategies but with bidirectional parsing. The comparison that
matters is A vs E (does bidirectionality help at all?) and B vs F (does it
compose with the arena optimization?).

Key metrics:
- **Wall time** (latency): should approach 0.5x of sequential on 2+ cores for
  large inputs with many top-level rules
- **CPU time** (throughput): should be ≥1x of sequential (bidirectional does
  at least as much total work due to merge overhead)
- **Crossover input size**: the input size below which goroutine spawn + merge
  cost exceeds the latency benefit

## Applicability

### Good candidates

- Document grammars with heading-delimited sections (organize text)
- Configuration grammars with distinct top-level blocks (TOML tables, INI
  sections)
- Data formats with repeated top-level elements (JSON arrays of objects,
  CSV rows, log files)

### Poor candidates

- Expression grammars (deeply nested, few top-level siblings)
- Grammars with single top-level rule that wraps everything (`Start <- Expr`)
- Small inputs where goroutine overhead dominates

### Non-candidates

- Grammars with left-recursive rules that change convergence behavior on
  reversal (until validated)
- Grammars with context-dependent parsing (not applicable to pure PEG)

## Open Questions

1. **Early termination protocol.** Option 1 (both parse to completion) is
   simpler but wastes CPU. Option 2 (cancellation on convergence) is more
   efficient but requires the VM to check an atomic variable periodically.
   The check frequency affects both overhead (too frequent = slow) and wasted
   work (too infrequent = late cancellation). Needs empirical tuning.

2. **Gap handling.** When neither parser reaches the middle, the fallback is
   sequential parsing of the gap region. Should the gap be parsed by the
   forward parser (continuing from where it left off), the backward parser
   (continuing from where it left off), or a third parser? The forward parser
   is the natural choice since it has accumulated the correct left context.

3. **N-way splitting.** The bidirectional model uses 2 parsers. Could it
   generalize to N parsers, each starting at `input[i * len/N]`? This
   requires interior starting points, which means the grammar must support
   parsing from a mid-input position — a much harder problem than starting
   from either end. The reversed grammar trick doesn't extend to N-way
   without additional analysis of mid-input anchor points (which is the
   separate anchor-scanning approach, not grammar reversal).

4. **Error reporting.** If the forward parser encounters an error at byte 50
   and the backward parser encounters an error at byte 950 (in a 1000-byte
   input), which error is more useful to the user? Forward errors are more
   natural ("expected X at line 3"). Backward errors may be confusing
   ("expected reversed X at reverse position 50"). The merge should prefer
   forward-parser errors and suppress backward-parser errors unless the
   forward parser didn't reach the error region.

5. **Reversed grammar as a debugging tool.** Even without parallel execution,
   the reversed grammar has diagnostic value. Parsing an input backward can
   reveal ambiguities where the grammar accepts the same input differently
   depending on direction. If `parse(input) ≠ translate(parse_rev(reverse(input)))`,
   the grammar has a directionality-dependent ambiguity. This could be surfaced
   as a diagnostic in the LSP (Language Server Protocol).

---

## Architectural Note: Direct Codegen Without the VM

This document describes generating a reversed grammar and feeding it through
the compilation pipeline to produce a second VM-interpreted parser. With direct
codegen, the reversed grammar goes through the emitter instead, producing a
`parseBackward` function set — one function per reversed rule, emitted as
native Go/Rust.

The simplification is substantial. The current architecture would require:

```
reversed AST → compiler → Program → encoder → Bytecode → GenGoEval (second VM)
```

Direct codegen:

```
reversed AST → emitter → parseBackward functions (native code)
```

No second bytecode blob, no second VM instance, no second instruction
dispatch loop. The forward and backward parsers are both native function
sets in the same package. The merge step calls them directly.

The cursor translation (forward vs. backward byte access) also simplifies.
Instead of abstracting byte access in the VM with a `cursorTranslator`
interface, the backward emitter generates functions where `input[pos]`
is emitted as `input[len(input)-1-pos]`. The direction is a compile-time
constant baked into every byte access — no runtime flag, no branch.

The cancellation-on-convergence mechanism (open question 1) becomes easier
with direct codegen: the emitter can insert an atomic check at the entry
point of each top-level rule function, rather than modifying the VM's inner
loop. The check is a single `atomic.LoadInt64` call at a natural function
boundary, not an instruction injected into the bytecode stream.
