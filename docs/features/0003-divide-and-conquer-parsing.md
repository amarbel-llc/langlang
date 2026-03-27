---
date: 2026-03-27
promotion-criteria: Scanner correctly identifies all structural junctions for
  JSON and langlang self-hosting grammars, and parallel parsing of independent
  regions produces identical results to sequential parsing.
status: proposed
---

# Divide-and-Conquer Parsing

## Problem Statement

PEG parsing is inherently sequential -- the parser processes input byte-by-byte
from left to right. For large inputs with recursive nesting structure (JSON
documents, configuration files, source code), this leaves multiple CPU cores
idle. Certain bytes in a grammar (structural delimiters like `{}`, `[]`, `,`)
can only appear at positions where the parser makes a routing decision. Finding
these "decision junctions" in a lightweight pre-scan reveals the parse tree's
branching skeleton, enabling N-way parallel parsing at every depth level.

## Interface

**Grammar analysis (codegen time):** The codegen walks the grammar AST and
extracts a `ScannerSpec` classifying which bytes are junctions
(open/close/separator), which define quoting contexts, and which are escape
prefixes.

**Scanner (runtime):** A single `O(n)` pre-scan with 1 bit of quoting state
produces a flat list of `JunctionHit{position, depth, kind}`. This skeleton
partitions the input into independent leaf regions.

**Parallel parsing:** Each leaf region is parsed independently by a standard PEG
parser instance. Results are stitched together using the skeleton's
depth/position metadata.

**CLI:** `langlang generate --parallel` flag enables D&C codegen. The generated
package includes the scanner, region parser, and merge logic.

``` go
func ParseParallel(input []byte) (*tree, error)   // new entry point
func ParseSequential(input []byte) (*tree, error)  // existing, unchanged
```

## Examples

For JSON input `{"name":"alice","items":[1,2,{"x":3}]}`, the scanner produces:

    { at 0   depth 0  (open)
    : at 6   depth 1  (separator)
    , at 14  depth 1  (separator)
    [ at 23  depth 1  (open)
    , at 25  depth 2  (separator)
    { at 28  depth 2  (open)
    } at 37  depth 2  (close)
    ] at 38  depth 1  (close)
    } at 40  depth 0  (close)

Leaf regions (string content, numbers, booleans) between junctions are parsed
independently.

## Limitations

- Grammars must have identifiable decision junctions (literal delimiters at
  structural positions). Expression-heavy grammars with few delimiters see
  minimal benefit.
- The scanner assumes junction bytes are unambiguous outside quoting contexts.
  Grammars where structural bytes can appear as content without quoting cannot
  use this optimization.
- Goroutine spawn + merge overhead means small inputs (\< \~10 KB) are faster
  with sequential parsing. The generated code includes a size threshold.
- Quoting contexts with escape sequences require the scanner to track escape
  state, adding complexity for grammars with multi-character escape sequences.

## More Information

- Full design with junction classification algorithm, merge strategies, and
  region parsing:
  [langlang-divide-and-conquer.md](../references/langlang-divide-and-conquer.md)
- Depends on: [FDR-0001 Typed Tree Extraction](0001-typed-tree-extraction.md)
  (extraction of parsed regions)
- Extended by: [FDR-0005 SIMD Junction Scanner](0005-simd-junction-scanner.md)
  (SIMD acceleration of the scanner)
