# Junction Scanner: Divide-and-Conquer Parsing

**Date:** 2026-03-27 **Status:** exploring **Scope:** JSON grammar
proof-of-concept

## Problem

PEG parsers are inherently sequential: each byte is processed through the
bytecode VM with backtracking, achieving \~30-35 MB/s on JSON. For large inputs,
this is the bottleneck. The grammar's structure contains information that could
be exploited to split input into independently-parseable regions.

## Solution

A two-phase approach:

1.  **Junction scan** (O(n), \~450-600 MB/s): Pre-scan input for structural
    delimiter bytes (`{}[],:` for JSON) derived from grammar analysis, tracking
    depth and quoting state. Produces a flat list of `JunctionHit` values.

2.  **Partition building**: Convert junction hits into a hierarchical tree of
    `Partition` regions bounded by matched open/close delimiters. Each partition
    can be parsed independently by the PEG parser.

3.  **Parallel parse**: Parse partitions concurrently across goroutines, then
    merge results.

## Current state (MVP)

- Grammar analyzer (`analyze.go`) derives `ScannerSpec` from PEG AST
- Scanner (`scan.go`) finds junctions with quoting/escape awareness
- Partition builder (`partition.go`) produces hierarchical partition tree
- Verification tests prove each partition parses independently to the same
  result as the full sequential parse
- Sequential partition parsing runs at \~93% of full parse throughput (overhead
  from scan + partition allocation)

## Future work

### Arena for scanner output

The scanner currently returns `[]JunctionHit` with per-call heap allocations.
Benchmarks show 167-9224 allocs/op depending on input size. An arena-based
approach (similar to the tree node arena) for `JunctionHit` slices and
`Partition.Children` would reduce GC pressure, especially important when
scanning feeds a parallel parse pipeline.

### Parallel partition parsing

Parse independent partitions across goroutines. The scan+partition phase is
\~15x faster than PEG parsing, so even with synchronization overhead, parallel
parsing should scale well on multi-core machines. Key design questions:

- Goroutine pool size vs. partition count
- Tree merging strategy (arena concatenation vs. copy)
- Parser instance pooling (each goroutine needs its own parser)

### Generalization beyond JSON

The grammar analyzer should work for any PEG grammar with structural delimiters.
Needs validation against additional grammars (XML, CSV, TOML, etc.).
