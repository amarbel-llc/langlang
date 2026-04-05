---
date: 2026-04-05
decision-makers: Sasha
status: accepted
---

# Use translation layer for tommy encode path, evolve to copy-on-write overlay

## Context and Problem Statement

Langlang's TOML grammar now produces CST-mode parse trees with named token nodes
(WS, Newline, Comment, Equals, BracketOpen, etc.) and byte-for-byte round-trip
fidelity. Tommy needs to consume these trees for format-preserving TOML editing,
but langlang trees are immutable flat arrays while tommy's encode path relies on
in-place mutation of `*cst.Node` trees (`SetValue`, `EnsureChildTable`,
`AppendArrayTableEntryAfter`). How should tommy's encode path work with
langlang's immutable parse trees?

## Decision Drivers

- Tommy's existing encode path (722 lines of accessors + 1122 lines of document
  API) works correctly and is well-tested against byte-for-byte round-trip
  conformance
- Langlang's tree is a flat `[]node` array with `uint32` byte offsets into the
  original input -- immutable by design, zero-copy, zero-allocation
- Tommy migration (#16) is a tracer bullet for all langlang downstream adoption
  -- risk must be minimized
- Tommy's `Document.Set()` must support structural mutations (create tables,
  append array-of-tables entries), not just value replacement

## Considered Options

- Option 1: Translation layer (`langlang.Tree` to `*cst.Node`)
- Option 2: Streaming writer (byte-offset splicing)
- Option 3: Copy-on-write overlay on immutable tree

## Decision Outcome

Chosen option: "Option 1: Translation layer" as the initial implementation,
because it validates the grammar and parse pipeline end-to-end with zero risk to
tommy's existing encode path. All existing mutation code, accessors, document
API, and codegen IR renderer work unchanged. Evolve to Option 3 (copy-on-write
overlay) once the parse side is stable and the translation layer's allocation
overhead becomes a bottleneck.

### Consequences

- Good, because tommy's entire existing encode stack works unchanged --
  `SetValue`, `EnsureChildTable`, `AppendArrayTableEntryAfter`, all 722 lines of
  accessors, marshal/unmarshal, codegen IR renderer
- Good, because the tracer bullet validates the grammar with minimal risk
- Good, because the translation layer is \~200 lines of straightforward
  node-kind mapping with `Trivia`/`LineEnd` flattening
- Bad, because every parse allocates a full `*cst.Node` tree on top of
  langlang's flat tree, partially defeating langlang's zero-allocation advantage
- Bad, because the translation layer is throwaway code if we evolve to Option 3
- Neutral, because the allocation overhead is acceptable for tommy's use case
  (config files, not hot-path parsing)

### Confirmation

The translation layer is confirmed working when tommy's conformance test suite
passes: `Parse(input) -> translate -> Bytes() == input` for all test cases, plus
`Set()` mutations preserve surrounding whitespace and comments.

## Pros and Cons of the Options

### Option 1: Translation layer

Walk the langlang tree after parsing and build a tommy-compatible `*cst.Node`
tree. Every langlang named node becomes a `*cst.Node` with the appropriate
`NodeKind`. Leaf text becomes `Raw []byte`. Flatten `Trivia` and `LineEnd`
wrapper nodes so their WS/Newline/Comment children become direct siblings.

- Good, because zero changes to tommy's encode path
- Good, because fastest path to production validation
- Good, because straightforward implementation (\~200 lines)
- Bad, because allocates a second tree on every parse
- Bad, because translation layer is throwaway if we evolve

### Option 2: Streaming writer

Don't build a mutable tree. Accumulate mutations as `map[key]newValue`. On
`Bytes()`, walk the langlang tree's byte offsets and splice: copy unchanged byte
ranges from `input[start:end]`, write new value bytes at modified positions.

- Good, because zero tree allocation
- Good, because minimal memory -- just the original input + change list
- Good, because round-trip fidelity is trivial for unchanged regions
- Bad, because structural mutations (create table, append array entry) require
  computing insertion points from immutable byte offsets
- Bad, because multiple overlapping mutations need conflict resolution
- Bad, because tommy's entire `accessors.go` and much of `document.go` would
  need rewriting (\~1800 lines)

### Option 3: Copy-on-write overlay

Keep the langlang tree immutable. Build a thin overlay tracking modifications
per `NodeID`: replacements (new bytes for a value node), insertions (synthetic
nodes at specific offsets), and deletions (nodes to skip). On `Bytes()`, walk
the tree and consult the overlay at each node.

- Good, because no full tree copy -- reads use the fast immutable tree
- Good, because mutations are O(1) to record
- Good, because `Bytes()` is a single linear walk
- Good, because natural architecture for immutable tree + selective mutation
- Bad, because novel pattern, not battle-tested
- Bad, because tommy's accessors need adaptation to consult the overlay
- Bad, because insertion ordering at the same offset needs careful handling

## Benchmark Results (Option 1 --- Translation Layer)

Measured on 11th Gen Intel i7-1165G7, `go/tomlcst/` benchmarks.

### Allocation optimization journey

  ---------------------------------------------------------------
  Step                            30kb allocs     500kb allocs
  ------------------------------- --------------- ---------------
  Original (no arena)             57,149          889,744

  \+ Arena allocator              11,718 (-79%)   184,439

  \+ Scratch mark/restore         8,521 (-85%)    134,772

  \+ ConcreteTree + iterator      6,601 (-88%)    104,970
  ---------------------------------------------------------------

### Final numbers

  ------------------------------------------------------------------------------
                           30kb input                 500kb input
  ------------------------ -------------------------- --------------------------
  **Parse only**           4.1 ms, 0 allocs           72 ms, 0 allocs

  **Translate only**       6.6 ms, 6.6k allocs        114 ms, 105k allocs

  **Parse + Translate**    10.7 ms, 6.6k allocs       186 ms, 105k allocs
  ------------------------------------------------------------------------------

### Key optimization techniques

1.  **Arena allocator** --- pre-allocate `[]Node` and `[]*Node` slices sized
    from `len(input)/3`. Nodes come from the flat array; child pointer slices
    share a single backing array. Eliminated 79% of allocs.

2.  **Scratch buffer with mark/restore** --- reusable `[]*Node` buffer for
    `translateChildren`. Mark position before recursion, restore after. Avoids
    per-call `var tmp []*Node` allocations. Another 6% reduction.

3.  **ConcreteTree type alias** --- exposed `tree` as `ConcreteTree` so
    `tomlcst` can type-assert and call `IterDirectChildren` on the concrete
    type. This allows the compiler to inline the iterator closure, avoiding heap
    escape. Another 3% reduction.

### Iterator escape analysis findings

`IterDirectChildren` returns an `iter.Seq[NodeID]` closure. Escape analysis
shows:

- **Direct call on `*tree`**: closure is inlined, zero allocs
- **Interface call on `Tree`**: closure escapes to heap (1 alloc per call)
  because the compiler cannot inline across interface dispatch
- **`ConcreteTree` type assert**: compiler devirtualizes and inlines, zero
  allocs --- same as direct call

This means `IterDirectChildren` should be preferred over `AppendChildren` when
the concrete tree type is available. `AppendChildren` remains on the `Tree`
interface for callers that cannot type-assert (e.g., generated parser code that
uses the interface).

`slices.Collect(tree.IterDirectChildren(id))` is the idiomatic way to get a
`[]NodeID` slice when indexed access is needed (used in `grammar_parser_v2.go`).

## Option 3 Analysis (Copy-on-Write Overlay)

Examined in detail (2026-04-05) after implementing Option 1. The COW overlay
would store `replacements map[NodeID][]byte`, `insertions []Insertion`, and
`deletions map[NodeID]bool` on top of the immutable langlang tree. `Bytes()`
would be a linear scan splicing replacement bytes at modified offsets.

### What works well

- `SetValue` maps cleanly: find the value NodeID, add to `replacements`
- `Bytes()` is a single linear scan over leaf byte ranges --- no tree needed
- Read-only accessors (`KeyValueName`, `KeyValueValue`) work directly on the
  immutable tree via `Tree.Visit()` + name matching

### What's hard

- **Structural insertions** (`EnsureChildTable`, `AppendArrayTableEntryAfter`)
  require computing exact byte offsets for insertion points and synthesizing raw
  bytes with correct formatting (brackets, newlines, indentation)
- **Reading modified state** --- after `Set("key", "new")`, `Get("key")` must
  check `replacements` before falling through to the tree
- **Navigation API mismatch** --- tommy walks `container.Children` as a slice;
  the overlay would need to simulate this over the immutable tree's `Visit()` +
  `Children()` API, a fundamentally different shape
- **Multiple mutations at same offset** --- insertions need stable ordering;
  deletions + insertions at the same node need careful sequencing

### Conclusion

The COW approach is viable but amounts to building a **new document API** on top
of the immutable tree rather than adapting tommy's existing one. Every accessor
and mutator in tommy's 722-line `accessors.go` and 1122-line `document.go` would
need reimplementation. This is the right optimization target once allocation
overhead becomes a measurable bottleneck in real usage, but premature for the
tracer bullet phase.

## More Information

- GitHub issue: amarbel-llc/langlang#16 (tommy migration tracer bullet)
- CST-mode grammar: `grammars/toml.peg` (commit 2796b45)
- Translation layer: `go/tomlcst/` (commit 9316ccc)
- Round-trip tests: `go/tests/toml/toml_test.go`, `go/tomlcst/translate_test.go`
- Tommy's CST: `pkg/cst/node.go`, `pkg/cst/accessors.go`,
  `pkg/document/document.go` in amarbel-llc/tommy
- Related FDRs: FDR-0001 (typed tree extraction), FDR-0002 (typed arena
  strategies)
