---
date: 2026-03-27
promotion-criteria: Benchstat comparison of at least two strategies against
  baseline shows statistically significant results (p \< 0.05) on medium JSON
  and expression inputs, with clear winner identified.
status: experimental
---

# Typed Arena Strategies

## Problem Statement

The baseline extraction path (FDR-0001) builds a generic tree in one arena, then
walks it to produce typed output -- two data structures in memory. Alternative
arena strategies could fuse parsing and extraction, reducing allocations and GC
pressure. The question is whether the added complexity of typed arenas justifies
the performance gain, and which strategy offers the best trade-off.

## Interface

Three strategies benchmarked against the baseline, using standard Go
`testing.B`:

- **Strategy A (byte buffer + offset refs):** Single `[]byte` bump allocator.
  Typed slots store `(offset, length)` pairs. Materialization at read time. Uses
  `unsafe.Pointer` for integer writes only.
- **Strategy B (typed parallel arenas):** Separate `[]T` slice per grammar rule
  kind. Compound `NodeID` encodes `(arenaKind, index)`. Zero `unsafe`, fully
  idiomatic Go.
- **Strategy C (byte buffer + unsafe.String/Slice):** Same byte buffer as A, but
  reads alias arena memory directly. No materialization copy. Borrowed-output
  semantics (invalidated on next parse).

**Preference order:** B \> A \> C (safety and maintainability over raw
performance).

**Benchmark grammars:** JSON (wide/shallow), recursive expressions
(deep/narrow).

**Input sizes:** \~100 bytes, \~10 KB, \~1 MB per grammar.

**Primary metrics:** `ns/op`, `allocs/op`, `B/op`. Secondary: parse-only time,
extraction-only time, peak RSS, backtrack cost.

## Examples

``` sh
go test -bench=. -benchmem -count=10 ./baseline/ > baseline.txt
go test -bench=. -benchmem -count=10 ./strategy_b/ > strategy_b.txt
benchstat baseline.txt strategy_b.txt
```

**Viability threshold:** \>=20% reduction in `ns/op` on medium inputs (both
grammars), or \>=50% reduction in `allocs/op`.

**Rejection criteria:** Parse-only overhead exceeds extraction savings, \>=2x
backtrack regression, or \>=2x peak RSS.

## Limitations

- Benchmarks measure VM-based parsing only. Direct codegen (no VM) changes the
  landscape -- arena strategies shift from "tree node allocation" to
  "intermediate state allocation."
- x86-64 only for initial benchmarks; ARM64 characteristics deferred.
- Does not measure GC interaction under concurrent load.

## More Information

- Full benchmark methodology and analysis criteria:
  [langlang-arena-benchmark-plan.md](../references/langlang-arena-benchmark-plan.md)
- Depends on: [FDR-0001 Typed Tree Extraction](0001-typed-tree-extraction.md)
  (baseline extraction)
