# Typed Arena Extraction: Benchmark Plan

## Goal

Measure the performance characteristics of three typed arena strategies against
the baseline (post-parse tree walk) to determine which approach, if any,
justifies the added complexity over Level 0 extraction.

## Strategies Under Test

**Baseline (Level 0):** Generic `tree` arena → post-parse extraction walk with
arena-direct access and constant-folded name IDs. Two data structures in memory:
the generic tree and the typed output.

**Strategy A (byte buffer + offset refs):** Single `[]byte` bump allocator. Typed
slots store `(offset, length)` integer pairs. Materialization into Go strings
and slices happens once at read time via copying. No `unsafe` for Go runtime
types; `unsafe.Pointer` used only for writing fixed-width integers into the byte
buffer.

**Strategy B (typed parallel arenas):** Separate `[]T` slice per grammar rule
kind. Compound `NodeID` encodes `(arenaKind, index)`. Rewind restores each
slice's length independently. Zero `unsafe`. Fully idiomatic Go.

**Strategy C (byte buffer + unsafe.String/unsafe.Slice):** Same byte buffer as
Strategy A, but final read produces Go strings and slices that alias the arena
memory directly via `unsafe.String` and `unsafe.Slice`. No materialization copy.
Borrowed-output semantics (invalidated on next parse).

## Grammars

Use two grammars that exercise different parse tree shapes:

**JSON (wide and shallow):** High fan-out arrays and objects, many terminal
string/number nodes, moderate nesting depth. Stresses allocation throughput for
small homogeneous nodes. The grammar already exists in `grammars/json.peg`.

**A recursive expression grammar (deep and narrow):** Deeply nested binary
expressions with left recursion and precedence climbing. Stresses the
backtracking rewind path and produces tall, thin trees. Use or adapt
`grammars/langlang.peg` (self-hosting grammar) or write a minimal arithmetic
grammar.

## Inputs

For each grammar, three input sizes:

- **Small:** ~100 bytes. A short JSON object or expression. Measures per-call
  overhead where setup cost dominates.
- **Medium:** ~10 KB. A realistic document. The expected hot path.
- **Large:** ~1 MB. Stress test. Measures allocation scaling and GC pressure.

Generate inputs deterministically from a seed so benchmarks are reproducible.
For JSON, use nested objects/arrays with string and number values. For
expressions, generate balanced and left-skewed trees.

## Metrics

### Primary

1. **Wall time per parse+extract** (`ns/op`). The single number that matters
   most. Measured as the total time from input bytes to a fully materialized
   typed Go struct.

2. **Allocations per operation** (`allocs/op`). Measures GC pressure. The
   baseline (Level 0) has a known allocation profile: one `tree.Copy()` plus one
   typed struct tree. Strategies A and B should reduce this; Strategy C should
   minimize it.

3. **Bytes allocated per operation** (`B/op`). Total heap bytes. Distinguishes
   between "fewer allocations of the same size" and "genuinely less memory."

### Secondary

4. **Parse-only time** (no extraction). Isolates the overhead that the typed
   arena strategies add to the parse phase itself. If Strategy B's compound
   `NodeID` or Strategy A's byte-buffer writes slow down parsing, this metric
   surfaces it. Measured by parsing the same input with the unmodified generic
   tree.

5. **Extraction-only time** (Level 0 only). Time spent in the post-parse walk,
   measured by parsing once (outside the benchmark loop) and then benchmarking
   the extraction function alone. Establishes the upper bound of what
   Strategies A/B/C can save.

6. **Peak RSS (Resident Set Size) under sustained load.** Run 10K parses in a
   loop, measure peak memory. Detects whether arena reuse is effective or
   whether the GC is retaining abandoned arenas.

7. **Backtrack cost.** For the expression grammar, construct an input that
   triggers significant backtracking (e.g., deeply nested alternatives that
   fail before the last choice). Measure parse+extract time relative to an
   equivalent input that doesn't backtrack. This isolates the cost of the
   rewind path: zeroing in Strategy A, multi-slice truncation in Strategy B,
   and `truncateArena` in the baseline.

## Benchmark Structure

Each benchmark is a standard Go `testing.B` function. All strategies are
implemented as separate packages so the compiler can't inline across them (which
would invalidate the comparison).

```
benchmarks/
├── go.mod
├── inputs/
│   ├── generate.go          # deterministic input generation
│   └── inputs_test.go       # sanity check: generated inputs parse correctly
├── baseline/
│   ├── bench_test.go        # Level 0: generic tree + extraction walk
│   └── extract_generated.go # generated extraction code (from the plan)
├── strategy_a/
│   ├── bench_test.go        # byte buffer + offset refs
│   ├── arena.go             # typedArena implementation
│   └── parser_generated.go  # generated parser with capture handlers
├── strategy_b/
│   ├── bench_test.go        # typed parallel arenas
│   ├── arenas.go            # per-rule arena types
│   └── parser_generated.go
├── strategy_c/
│   ├── bench_test.go        # byte buffer + unsafe.String/Slice
│   ├── arena.go
│   └── parser_generated.go
├── parse_only/
│   ├── bench_test.go        # unmodified parser, no extraction (control)
│   └── parser_generated.go
└── analysis/
    └── compare.go           # reads benchstat output, produces summary table
```

## Benchmark Functions

Each `bench_test.go` contains the same set of functions, varying only the
implementation:

```go
func BenchmarkJSONSmall(b *testing.B)      { benchParse(b, jsonSmall) }
func BenchmarkJSONMedium(b *testing.B)     { benchParse(b, jsonMedium) }
func BenchmarkJSONLarge(b *testing.B)      { benchParse(b, jsonLarge) }
func BenchmarkExprSmall(b *testing.B)      { benchParse(b, exprSmall) }
func BenchmarkExprMedium(b *testing.B)     { benchParse(b, exprMedium) }
func BenchmarkExprLarge(b *testing.B)      { benchParse(b, exprLarge) }
func BenchmarkExprBacktrack(b *testing.B)  { benchParse(b, exprBacktrack) }
```

Each calls `b.ReportAllocs()` and uses `b.ResetTimer()` after any setup.

For the extraction-only benchmark (baseline package only):

```go
func BenchmarkExtractOnlyJSONLarge(b *testing.B) {
    tree := parseOnce(jsonLarge) // outside the loop
    b.ResetTimer()
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        _, _ = ExtractJSONValue(tree, root)
    }
}
```

For sustained-load RSS measurement:

```go
func TestPeakRSS(t *testing.T) {
    var m runtime.MemStats
    parser := newParser()
    for i := 0; i < 10_000; i++ {
        _ = parser.ParseAndExtract(jsonMedium)
    }
    runtime.GC()
    runtime.ReadMemStats(&m)
    t.Logf("HeapInuse: %d MB, HeapSys: %d MB", m.HeapInuse>>20, m.HeapSys>>20)
}
```

## Running

```sh
# Run all benchmarks, 10 iterations each for statistical stability
go test -bench=. -benchmem -count=10 ./baseline/ > baseline.txt
go test -bench=. -benchmem -count=10 ./strategy_a/ > strategy_a.txt
go test -bench=. -benchmem -count=10 ./strategy_b/ > strategy_b.txt
go test -bench=. -benchmem -count=10 ./strategy_c/ > strategy_c.txt
go test -bench=. -benchmem -count=10 ./parse_only/ > parse_only.txt

# Compare with benchstat
benchstat baseline.txt strategy_a.txt strategy_b.txt strategy_c.txt
```

## Analysis Criteria

The benchstat output produces a table with `ns/op`, `B/op`, and `allocs/op`
across all four implementations. Decision framework:

**Strategy is viable if** it shows a statistically significant improvement
(p < 0.05 per benchstat) in `ns/op` on the medium input for at least one
grammar, without regressing `allocs/op` or `B/op`.

**Strategy is preferred over Level 0 if** it meets one of:
- ≥20% reduction in `ns/op` on medium inputs (both grammars)
- ≥50% reduction in `allocs/op` on medium inputs (both grammars)
- ≥30% reduction in `B/op` on large inputs (either grammar)

**Strategy is rejected if** any of:
- Parse-only time (control) is faster than the strategy's parse+extract time
  by less than the extraction-only time. This means the strategy added more
  overhead to parsing than it saved by eliminating the extraction pass.
- Backtrack benchmark shows ≥2x regression vs baseline on the expression
  grammar. The rewind path is too expensive.
- Peak RSS under sustained load is ≥2x the baseline. Arena reuse is broken.

**Between A, B, and C:** If multiple strategies are viable, prefer in order:
B > A > C. B has no `unsafe` and is the most maintainable. A is simpler than B
but uses `unsafe` for integer writes. C has the strongest performance ceiling
but the weakest safety properties and the borrowed-output footgun.

## Implementation Order

1. Implement input generation (`inputs/generate.go`). Verify inputs parse
   correctly with the unmodified langlang parser.
2. Implement the baseline benchmark (Level 0 extraction from the plan
   document). This is the control.
3. Implement the parse-only benchmark (unmodified parser, no extraction).
   This establishes the floor.
4. Implement Strategy B first (typed parallel arenas). It's the safest and
   most idiomatic. If it meets the viability threshold, skip A and C.
5. If B is not viable, implement Strategy A (byte buffer). If A is viable,
   skip C.
6. If A is not viable, implement Strategy C (unsafe aliasing). If C is not
   viable, Level 0 is the answer and the typed arena idea is not worth the
   complexity.
7. Run the full comparison. Write up results.

## Deferred

- Benchmarking with CPU profiles (`-cpuprofile`) and memory profiles
  (`-memprofile`) for root-cause analysis of any surprising results.
- Benchmarking on GOARCH=arm64 (different cache line sizes, different branch
  prediction characteristics).
- Benchmarking with GOGC=off to isolate allocation cost from GC cost.
- Comparing against hand-written parsers (not PEG-generated) as an absolute
  performance ceiling.

---

## Architectural Note: Direct Codegen Without the VM

This benchmark plan assumes a VM-based parser producing a tree, with the arena
strategies governing how that tree is allocated. If the codegen path eliminates
the VM and emits direct parse functions from the grammar AST, the benchmark
landscape changes.

The baseline (Level 0) disappears as a distinct strategy — there is no
separate "parse then extract" step, because parsing and extraction are fused
into the generated functions. The arena strategies (A, B, C) shift from
"how to allocate tree nodes" to "how to allocate intermediate state during
parsing" — specifically, the backtracking state for ordered choices and the
accumulation buffers for repetitions.

For direct codegen, the Go call stack replaces the VM's backtracking stack.
Ordered choice becomes sequential `if` branches; repetition becomes `for`
loops with `append`. The relevant allocation question becomes: does the Go
compiler heap-allocate the intermediate slices for repetitions, or can it
stack-allocate them? This is determined by escape analysis, not by arena
strategy.

The benchmark plan remains useful for one scenario: comparing VM-interpreted
parsing (with different arena strategies) against direct codegen (which has
no arena at all). This comparison measures the total overhead of the VM
abstraction layer and validates whether eliminating it produces measurable
end-to-end improvement.

The parse-only benchmark (the control) becomes the most important metric: it
measures the floor for direct codegen and the ceiling for any VM-based
approach. If direct codegen's parse-only time is significantly lower than the
VM's, the arena strategies are optimizing the wrong layer.
