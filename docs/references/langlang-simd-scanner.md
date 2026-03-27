# SIMD Assembly Codegen via Avo

## Concept

Langlang's codegen already produces Go source files at `go generate` time. Avo
(`github.com/mmcloughlin/avo`) is a Go library that produces Go-compatible
assembly (`.s`) files at `go generate` time. The two chain naturally: langlang's
grammar analysis decides what operations are needed, avo decides how to express
them as x86-64 SIMD instructions. The output is architecture-specific assembly
with a scalar Go fallback for other platforms.

This is a targeted optimization, not a general strategy. It applies to one
specific function: the decision junction scanner from the divide-and-conquer
exploration. Other potential targets (arena field access, VM dispatch) have
smaller payoffs and higher complexity.

## Target: The Junction Scanner

The junction scanner's inner loop classifies each input byte into one of a
small number of categories: junction byte (structural delimiter), quote toggle,
escape prefix, or uninteresting. On scalar hardware, this is a switch statement
processing one byte per iteration. With SIMD, it processes 32 bytes (AVX2) or
64 bytes (AVX-512) per cycle using vectorized table lookup.

This is the same technique used by simdjson for JSON structural character
detection. The difference: simdjson's classification table is hardcoded for
JSON. Here, the langlang codegen generates the classification table from the
grammar's `ScannerSpec` — different grammars produce different tables but the
same SIMD skeleton.

### Expected speedup

Scalar scanner: ~2-4 ns/byte (one byte per loop iteration, branch-heavy).

AVX2 scanner: ~0.15-0.3 ns/byte (32 bytes per iteration, branchless
classification). This is 8-16x faster.

AVX-512 scanner: ~0.08-0.15 ns/byte (64 bytes per iteration). 16-32x faster.

For a 1 MB input: scalar ~2-4 ms, AVX2 ~150-300 μs, AVX-512 ~80-150 μs.

## How VPSHUFB Classification Works

`VPSHUFB` (Packed Shuffle Bytes) is a single instruction that performs 32
simultaneous table lookups. Each byte of the input selects a value from a
16-entry lookup table. The trick: use the low nibble (4 bits) of each input
byte as the table index, producing a bitmask that encodes which category the
byte belongs to.

For JSON, the interesting bytes and their low nibbles:

| Byte | Hex  | Low nibble | Category       |
|------|------|------------|----------------|
| `{`  | 0x7B | 0xB        | Open           |
| `}`  | 0x7D | 0xD        | Close          |
| `[`  | 0x5B | 0xB        | Open           |
| `]`  | 0x5D | 0xD        | Close          |
| `,`  | 0x2C | 0xC        | Separator      |
| `:`  | 0x3A | 0xA        | Separator      |
| `"`  | 0x22 | 0x2        | Quote toggle   |
| `\`  | 0x5C | 0xC        | Escape prefix  |

Problem: `,` (0x2C) and `\` (0x5C) share low nibble 0xC. Also `{`/`[` share
0xB and `}`/`]` share 0xD. A single VPSHUFB can't distinguish them.

Solution: two-pass classification. First VPSHUFB uses the low nibble, second
VPSHUFB uses the high nibble. AND the results — a byte is classified as
interesting only if both nibble lookups agree.

```
Low-nibble table (16 entries):
  index 0x2 → bit 0 (quote candidate)
  index 0xA → bit 1 (separator candidate)
  index 0xB → bit 2 (open candidate)
  index 0xC → bit 3 (separator-or-escape candidate)
  index 0xD → bit 4 (close candidate)
  all others → 0

High-nibble table (16 entries):
  index 0x2 → bit 0 (matches " at 0x22)
  index 0x3 → bit 1 (matches : at 0x3A)
  index 0x5 → bit 2 | bit 3 | bit 4 (matches [ ] \)
  index 0x7 → bit 2 | bit 4 (matches { })
  all others → 0

Classification = low_result AND high_result
```

After the AND, each byte position has a nonzero value only if it's a
structurally interesting byte. `VPMOVMSKB` extracts a 32-bit mask of which
positions are nonzero. Each set bit is a junction or quote/escape byte that
needs processing.

The sparse-bit processing (iterating set bits in the mask) uses `TZCNT` (count
trailing zeros) to find the next set bit and `BLSR` (reset lowest set bit) to
clear it — both single-cycle instructions on modern x86.

## Grammar-Derived Lookup Tables

The lookup tables are constants derived from the grammar's `ScannerSpec`. The
codegen builds them at generation time:

```go
func buildLookupTables(spec ScannerSpec) (low, high [16]byte) {
    // For each junction, quote, and escape byte in the spec,
    // set the corresponding bits in the low-nibble and high-nibble tables.
    for _, j := range spec.Junctions {
        for _, b := range j.Bytes {
            loNib := b & 0x0F
            hiNib := b >> 4
            bit := nextFreeBit()
            low[loNib] |= bit
            high[hiNib] |= bit
        }
    }
    for _, q := range spec.Quotes {
        for _, b := range q.Open {
            // ... same pattern
        }
        for _, b := range q.Close {
            // ... same pattern
        }
        for _, b := range q.EscapePrefix {
            // ... same pattern
        }
    }
    return
}
```

Different grammars produce different table contents. The SIMD skeleton
(load → VPSHUFB × 2 → AND → VPMOVMSKB → sparse iteration) is identical.

### Bit budget

Each classification category needs one bit. VPSHUFB produces 8 bits per byte.
With the two-pass nibble technique, you have 8 usable classification bits.
This supports up to 8 distinct byte categories — more than enough for any
grammar's junction/quote/escape set.

If a grammar has more than 8 distinct interesting bytes (unlikely but
possible), the codegen falls back to scalar classification for that grammar.

## Avo Generator Structure

The codegen produces an avo generator program — a Go file with a `main()`
that calls avo's API to emit assembly. This program is itself generated from
the grammar analysis.

```
grammar.peg
    ↓  langlang grammar analysis → ScannerSpec
    ↓  buildLookupTables(spec) → low[16], high[16]
    ↓
    emit gen_scanner_asm.go:
        //go:build ignore
        package main

        import . "github.com/mmcloughlin/avo/build"

        func main() {
            genScanJunctions(low, high)  // constants baked in
            Generate()
        }
```

The `go generate` chain:

```go
// In the consumer package:

//go:generate langlang extract -grammar=json.peg
// → produces json_extract.go, gen_scanner_asm.go, scanner_generic.go

//go:generate go run gen_scanner_asm.go -out scanner_amd64.s -stubs scanner_stub_amd64.go
// → produces scanner_amd64.s, scanner_stub_amd64.go
```

Two `go generate` passes. The first produces Go source and the avo generator.
The second runs the avo generator to produce assembly.

## Generated Assembly Skeleton

The avo generator emits a function with this structure:

```
TEXT ·scanJunctionsAVX2(SB), NOSPLIT, $0-N

    // Load arguments: input base, input length, output base
    // Load lookup tables into YMM registers (from rodata)

main_loop:
    // Check: 32+ bytes remaining?
    CMP remaining, 32
    JL  tail

    // Load 32 bytes of input
    VMOVDQU (input_ptr), Y0

    // Extract low nibbles: Y1 = Y0 AND 0x0F (broadcast)
    VPAND   Y_MASK_0F, Y0, Y1

    // Extract high nibbles: Y2 = (Y0 >> 4) AND 0x0F
    VPSRLW  $4, Y0, Y2
    VPAND   Y_MASK_0F, Y2, Y2

    // Classify via lookup: Y3 = low_table[Y1], Y4 = high_table[Y2]
    VPSHUFB Y1, Y_LOW_TABLE, Y3
    VPSHUFB Y2, Y_HIGH_TABLE, Y4

    // Intersect: Y5 = Y3 AND Y4
    VPAND   Y3, Y4, Y5

    // Extract bitmask of nonzero bytes
    VPMOVMSKB Y5, mask_reg

    // If no interesting bytes in this chunk, advance and loop
    TEST    mask_reg, mask_reg
    JZ      advance

    // Process each set bit
process_bits:
    TZCNT   mask_reg, bit_pos       // position of lowest set bit
    BLSR    mask_reg, mask_reg      // clear that bit

    // Determine category from Y5 at bit_pos (load byte from Y5)
    // Compute depth adjustment (open: +1, close: -1, other: 0)
    // Write junction hit to output
    // ...

    TEST    mask_reg, mask_reg
    JNZ     process_bits

advance:
    ADD     $32, input_ptr
    SUB     $32, remaining
    JMP     main_loop

tail:
    // Process remaining <32 bytes with scalar loop
    // (reuse the generic Go implementation, or inline scalar)
    // ...

done:
    // Store output count
    RET
```

### Quoting state across chunks

The SIMD classification finds *candidate* quote/escape/junction bytes. But
the scanner also tracks quoting state (inside-string or not). This state
carries across 32-byte chunks.

Two approaches:

**Approach A: prefix-sum on quote toggles.** Count `"` bytes in each chunk
using `VPMOVMSKB` on the quote-bit mask. The parity (even/odd count) determines
whether the chunk ends inside or outside a string. XOR with the carry-in state
from the previous chunk. This is `O(1)` per chunk and fully vectorized. simdjson
uses this technique.

The complication: escape sequences (`\"`) suppress the quote toggle. Before
counting quotes, mask out escaped quotes. An escaped quote is a `"` preceded
by an odd number of `\` characters. Detecting this requires a carry-propagation
step on the backslash mask — also vectorizable via prefix operations.

**Approach B: scalar post-processing of candidates.** The SIMD pass produces
a flat list of candidate positions and their classification bits. A scalar
pass iterates this list, tracking quoting state, and filters out candidates
that are inside strings. Since the candidate list is much shorter than the
input (only structural bytes), this scalar pass is fast.

Approach B is simpler to implement and still achieves most of the speedup,
since the SIMD pass (which dominates for large inputs) is unchanged. Approach
A is what simdjson does and is faster for inputs where string content dominates
(many quoted regions, few structural bytes).

For the initial implementation, use Approach B. It's correct and simple. If
benchmarks show the scalar post-processing is a bottleneck, switch to
Approach A.

## Build Tags and Fallback

```
scanner_generic.go        // //go:build !amd64
scanner_stub_amd64.go     // //go:build amd64 — Go stubs for asm functions
scanner_amd64.s           // //go:build amd64 — AVX2 assembly

gen_scanner_asm.go        // //go:build ignore — avo generator (not compiled)
```

The `scanner_generic.go` file contains the scalar scanner from the D&C
exploration. On ARM64, MIPS, WASM, or any non-amd64 target, the scalar
version is used automatically. No conditional compilation in user code.

### Runtime feature detection

AVX2 is not available on all x86-64 CPUs (pre-Haswell, ~2013). The assembly
function should have a Go-level feature gate:

```go
// scanner_stub_amd64.go (generated by avo)

func scanJunctionsAVX2(input []byte, hits []junctionHit) int

// scanner_amd64.go (handwritten, not generated)

import "golang.org/x/sys/cpu"

func scanJunctions(input []byte) []junctionHit {
    hits := make([]junctionHit, 0, len(input)/10) // estimate
    if cpu.X86.HasAVX2 {
        n := scanJunctionsAVX2(input, hits[:cap(hits)])
        return hits[:n]
    }
    return scanJunctionsScalar(input)
}
```

The `cpu.X86.HasAVX2` check is performed once at call time, not per byte. The
branch predictor will lock onto whichever path the hardware supports.

## Scope and Constraints

### x86-64 only

Avo does not support ARM64. There is no mature equivalent for ARM NEON/SVE
assembly generation in Go. ARM64 users get the scalar fallback. If an ARM64
avo-like library emerges, the same codegen approach applies — the grammar
analysis and lookup table generation are architecture-independent; only the
instruction emission layer changes.

### One function

This optimization applies to the junction scanner and nothing else in the
near term. The VM dispatch loop and arena field access are possible future
targets, but their payoff is smaller and their implementation complexity is
substantially higher. The scanner is a self-contained, pure function with no
side effects and a simple interface (`[]byte → []junctionHit`), making it
ideal for isolated assembly optimization.

### Avo is experimental

Avo's API is not stable. The generated assembly is stable (it's just Plan 9
assembly text), but the Go API for producing it may change. Pin the avo
dependency version. If avo breaks, the scalar fallback is always available —
delete the `.s` file and the stubs, and the `!amd64` build tag picks up the
generic implementation.

## Relationship to Other Explorations

This document describes a performance optimization for one component of the
divide-and-conquer exploration (`langlang-divide-and-conquer.md`). It does not
affect the typed arena strategies (`langlang-arena-benchmark-plan.md`) or
bidirectional parsing (`langlang-bidirectional-parsing.md`).

The dependency chain is:

```
langlang-extract-plan.md          (extraction codegen — foundation)
    ↓
langlang-arena-benchmark-plan.md  (arena strategy selection — orthogonal)
    ↓
langlang-divide-and-conquer.md    (D&C parsing via junction scanning)
    ↓
this document                     (SIMD acceleration of the scanner)
```

This optimization is worth pursuing only after the D&C scanner exists and has
been benchmarked in scalar form. If the scalar scanner is already fast enough
(i.e., scan time is <5% of total parse+extract time), SIMD acceleration is
not worth the complexity.

---

## Architectural Note: Direct Codegen Without the VM

The SIMD scanner is already independent of the VM — it's a pre-parse step
that produces junction positions, not a replacement for the PEG parser. The
VM-vs-direct-codegen question doesn't affect this document's content.

However, in a direct codegen architecture, the scanner becomes even more
valuable. Without the VM interpreter's overhead, the PEG parsing of leaf
regions is faster, which means the scanner's cost is a larger fraction of
total time. The crossover point where SIMD acceleration matters shifts
downward — it becomes relevant for smaller inputs because the parse phase
itself is cheaper.

The avo-generated scanner and the direct-codegen parser compose cleanly:
both are native functions in the consumer's package, both generated at
`go generate` time, neither involves the VM. The scanner finds structure;
the parser functions handle content. No bytecode, no interpreter, no
dispatch overhead anywhere in the pipeline.
