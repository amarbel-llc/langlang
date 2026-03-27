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

## Current state

- Grammar analyzer (`analyze.go`) derives `ScannerSpec` from PEG AST
- Scanner (`scan.go`) finds junctions with quoting/escape awareness
- Partition builder (`partition.go`) produces hierarchical partition tree
- Parallel parser (`parallel.go`) dispatches partitions across GOMAXPROCS
  workers with `sync.Pool`-based parser instance reuse
- Verification tests prove each partition parses independently to the same
  result as the full sequential parse, both sequentially and in parallel

### Benchmarks (Apple M2 Pro, 12 cores)

Scanner phases:

  Phase              30kb   500kb   2000kb
  ---------------- ------ ------- --------
  ScanOnly            636     604      598
  Scan+Partition      566     532      527

All values in MB/s.

Parse comparison:

  Approach                     30kb   500kb   2000kb
  ------------------------- ------- ------- --------
  Full PEG parse               31.3    35.2     34.5
  Sequential partitions        29.4    32.8     31.8
  **Parallel partitions**     104.4   155.1    170.4

All values in MB/s. Parallel speedup vs full parse: 3.3x (30kb), 4.4x (500kb),
4.9x (2000kb). Scales with input size because larger documents produce more
depth-1 partitions for parallel work distribution.

### Limitations

The parallel parse currently only covers depth-1 child partitions (nested
containers within the root). Flat content between partitions (key-value pairs at
depth 0) is not yet parsed in the parallel pipeline. A complete solution needs
to handle inter-partition regions to produce a full parse tree.

## Future work

### Arena for scanner output

The scanner currently returns `[]JunctionHit` with per-call heap allocations.
Benchmarks show 167-9224 allocs/op depending on input size. An arena-based
approach (similar to the tree node arena) for `JunctionHit` slices and
`Partition.Children` would reduce GC pressure, especially important when
scanning feeds a parallel parse pipeline.

### Complete parallel parse tree

Handle flat content between partitions (e.g., string keys, scalar values at the
root depth) so the parallel pipeline produces a complete parse tree equivalent
to the full sequential parse. Options:

- Parse inter-partition regions on the main goroutine while workers handle
  containers
- Extend the partition model to include separator-delimited regions as parse
  units

### Tree merging

Parallel parsing produces independent per-partition trees. Merging them into a
single tree requires either arena concatenation (fast, requires offset fixup) or
a copy pass. The arena approach aligns with the existing tree architecture.

### Generalization beyond JSON

The grammar analyzer should work for any PEG grammar with structural delimiters.
Needs validation against additional grammars (XML, CSV, TOML, etc.).
