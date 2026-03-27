---
date: 2026-03-27
promotion-criteria: Property-based tests confirm parse equivalence between
  forward and backward parsers for at least two grammars (JSON and a
  repetition-heavy grammar), and wall time approaches 0.5x sequential on 2+
  cores for large inputs.
status: proposed
---

# Bidirectional PEG Parsing

## Problem Statement

For grammars with multiple top-level rules or repeated top-level elements, a
single PEG parser leaves one core idle while processing a large input
sequentially. A grammar that describes valid input forward can be mechanically
reversed to describe the same structure backward. Two parsers -- one running the
original grammar forward from byte 0, the other running the reversed grammar
backward from the last byte -- can parse the same input concurrently, converging
toward the middle.

## Interface

**Grammar reversal (codegen time):** A recursive AST visitor produces a reversed
`DefinitionNode` for each rule. Sequences reverse order, literals reverse bytes,
rule references point to `_rev` variants. Single-byte constructs (charsets,
ranges, any) are unchanged. Choice priority order is preserved.

**Reversibility validation:** The codegen classifies each rule as `ReverseSafe`,
`ReverseNeedsReview`, or `ReverseUnsafe`. Unsafe rules cause bidirectional mode
to be skipped. Rules needing review are treated as boundaries -- the backward
parser stops before entering them.

**Execution:** Two goroutines with independent VM instances and arenas, no
shared state. Results merge based on byte positions.

``` go
func ParseBidirectional(input []byte) (*tree, error)  // new entry point
```

**CLI:** `langlang generate --bidi` flag enables bidirectional codegen. Emits
both forward and backward parser code.

**Merge outcomes:** - Clean split: `fwd.consumed + bwd.consumed == len(input)`
-- concatenation - Overlap: both parsed the middle -- verify and deduplicate -
Gap: neither reached the middle -- forward parser fills the gap sequentially

## Examples

    input bytes: [  0  1  2  3  4  5  6  7  8  9  ]

    forward:      -->-->-->-->-->--> consumed: 6
    backward:                  <--<--<--<--<-- consumed: 4

For `Document <- Block*`, forward produces the first K blocks, backward produces
the last M blocks (in reverse order). Merge concatenates with the backward list
reversed.

## Limitations

- Latency optimization only -- total CPU is \>=1x sequential (both parsers do
  real work plus merge overhead). Wall time approaches 0.5x on 2+ cores for
  large inputs with many top-level rules.
- Poor candidates: expression grammars (deeply nested, few top-level siblings),
  grammars with a single wrapping start rule, small inputs where goroutine
  overhead dominates.
- Left-recursive rules change convergence behavior on reversal; these require
  validation before enabling.
- Backward parser error messages are confusing to users; merge prefers
  forward-parser errors.

## More Information

- Full design with reversal rules, cursor translation, merge algorithms, and
  stopping conditions:
  [langlang-bidirectional-parsing.md](../references/langlang-bidirectional-parsing.md)
- Orthogonal to: [FDR-0003 Divide-and-Conquer
  Parsing](0003-divide-and-conquer-parsing.md) (D&C splits at every depth via
  junction scanning; bidirectional splits at top level via grammar reversal)
