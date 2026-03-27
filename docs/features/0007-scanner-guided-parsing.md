---
date: 2026-03-27
promotion-criteria: Prototype demonstrates measurable reduction in backtracking
  and speculative matching on at least one grammar (JSON or Go), with
  correctness verified against sequential parse output.
status: exploring
---

# Scanner-Guided Parsing

## Problem Statement

PEG parsing spends most of its time on control flow overhead -- rule dispatch,
backtracking, speculative matching, and memoization lookups -- not on byte-level
comparisons. The divide-and-conquer approach (FDR-0003) parallelizes parsing
across independent regions, but each partition's parser still performs full
speculative matching internally. The junction scanner already knows where every
delimiter-bounded region starts and ends. Feeding that knowledge into the parser
would let it skip trial-parsing for delimiter-bounded constructs entirely,
eliminating the dominant source of wasted work without increasing parallelism
overhead.

## More Information

- Builds on: [FDR-0003 Divide-and-Conquer
  Parsing](0003-divide-and-conquer-parsing.md) (junction scanner produces the
  structural skeleton this feature consumes)
- Design doc with benchmark data:
  [langlang-divide-and-conquer.md](../references/langlang-divide-and-conquer.md)
