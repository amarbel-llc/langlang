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

## Experimental Results: Strategy B

Strategy B was hand-implemented for the JSON grammar (`json-extract` example)
with the following design:

- **Per-type arenas:** `JSONArenas` struct with `[]JSONValue`, `[]JSONObject`,
  `[]JSONMember`, `[]JSONArray` slices, plus flat backing buffers `MemberBuf`,
  `ItemBuf`, and `StringBuf` for child slices and string pointers.
- **Pre-count pass:** A single `Visit()` traversal counts nodes per nameID (zero
  allocs, map-free switch on known constants). Arena slices pre-allocated to
  exact capacity for pointer stability.
- **Slot reservation:** Each container (Object, Array) counts its direct
  children, reserves contiguous slots in the backing buffer, then fills them.
  Prevents interleaving from nested extraction.
- **String arena:** `*string` fields point into `StringBuf` instead of
  heap-allocating individual strings.

### Allocation results

  Method              30kb allocs   500kb allocs
  ------------------- ------------- --------------
  Heap extract        1,660         43,831
  Interface extract   3,638         87,657
  **Arena extract**   **7**         **7**

Arena reduces allocations by **99.6%** --- the 7 remaining allocs are the
`make()` calls for the arena slices themselves.

### Wall time results

  Method              30kb ns/op   500kb ns/op
  ------------------- ------------ -------------
  Heap extract        2.3M         44M
  Interface extract   2.3M         47M
  **Arena extract**   **2.6M**     **52M**

Arena is **\~10-15% slower** in wall time despite near-zero allocations.

### Diagnosis: where the time goes

Isolating extraction from parsing reveals the bottleneck:

  Benchmark              30kb ns/op   allocs
  ---------------------- ------------ --------
  Extract-only (heap)    155k         1,660
  Extract-only (arena)   516k         7
  Pre-count only         365k         0

The **pre-count pass alone** (365k ns) costs more than the entire heap
extraction (155k ns). Additionally, each Object and Array does a child-counting
`Visit()` to reserve contiguous slots in the backing buffer, adding a second
traversal per container. The heap path does a single `Visit()` per node with no
pre-counting.

### Root cause

The arena approach trades **allocation count** for **traversal count**. The
pre-count walk is O(n) over all nodes (same work as the extraction walk itself),
and per-container child counting adds another O(children) pass per Object/Array.
The Go allocator's bump-pointer fast path for small objects is fast enough that
1,660 heap allocations cost less than one additional full tree traversal.

### Possible improvements (not yet implemented)

1.  **Fold child counts into the pre-count pass.** Track parent context during
    the global pre-count walk to produce per-instance child counts, eliminating
    the per-container counting Visit. Requires outputting a parallel array of
    child counts indexed by node position.

2.  **Unsafe string aliasing.** Replace `StringBuf` with direct `unsafe.String`
    pointers into the parse tree's input buffer, eliminating even the string
    copy. Borrowed-output semantics (invalidated when input is freed).

3.  **Arena reuse across parses.** `Reset()` clears arenas without releasing
    memory. Under sustained load (many parses of similar-sized inputs), the
    arena slices stay warm and the 7 `make()` calls amortize to zero.

### Conclusion

Strategy B **meets the allocs/op viability threshold** (99.6% reduction, far
exceeding the 50% target) but **does not meet the ns/op threshold** (10-15%
slower, not 20% faster). The arena approach is viable for GC-sensitive workloads
(concurrent parsers, long-lived processes) where allocation count matters more
than single-parse latency. For single-parse throughput, the baseline heap
extraction is faster.

The per-container child counting overhead is the primary obstacle. Improvement
#1 (folding child counts into the pre-count) would eliminate the dominant cost
and may flip the wall-time result.

## Limitations

- Benchmarks measure VM-based parsing only. Direct codegen (no VM) changes the
  landscape -- arena strategies shift from "tree node allocation" to
  "intermediate state allocation."
- x86-64 only for initial benchmarks; ARM64 characteristics deferred.
- Does not measure GC interaction under concurrent load, where the alloc
  reduction would show its full benefit.

## More Information

- Full benchmark methodology and analysis criteria:
  [langlang-arena-benchmark-plan.md](../references/langlang-arena-benchmark-plan.md)
- Strategy B implementation: `go/examples/json-extract/json_types_arena.go`,
  `go/examples/json-extract/json_extract_arena.go`
- Depends on: [FDR-0001 Typed Tree Extraction](0001-typed-tree-extraction.md)
  (baseline extraction)
