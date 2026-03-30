---
date: 2026-03-27
promotion-criteria: Generated extraction code compiles and passes round-trip
  tests against grammars/json.peg with arena-direct access showing measurable
  speedup over interface-path extraction.
status: testing
---

# Typed Tree Extraction

## Problem Statement

Langlang parsers produce a generic `Tree` backed by an arena. Consumers who want
typed Go data must hand-write tree-walking code: checking `Name()` against
strings, navigating `Children()` by index, and converting `Text()` to typed
values. This boilerplate is proportional to grammar complexity, error-prone due
to positional indexing, and pays for interface dispatch on every node access
even though the grammar structure is statically known.

## Interface

A `//go:generate langlang extract` directive reads both the Go struct
definitions (via `go/ast` + `go/types`) and the PEG grammar (via `QueryAST` +
`QueryBytecode`), then emits extraction functions that index directly into the
tree's arena slices.

**Struct tagging:**

``` go
//go:generate langlang extract -grammar=json.peg

type JSONValue struct {
    Object *JSONObject `ll:"Object"`
    String *string     `ll:"String"`
}
```

**Generated output:** `<source>_extract.go` containing
`Extract<Type>(t *tree, id NodeID) (<Type>, error)` functions with:

- Constant-folded `nameID` checks (no string comparisons)
- Arena-direct field access (no interface dispatch)
- Dead child elimination (structural punctuation like `'='` skipped)

**CLI:** `langlang extract -grammar=path/to/grammar.peg` (new subcommand). Reads
`$GOFILE` from `go generate` environment.

**Parser addition:** `ParseRaw() (*tree, int, error)` method on generated
parsers for concrete-type access.

## Examples

Given a grammar:

``` peg
Assignment <- Identifier '=' Value
Identifier <- [a-zA-Z]+
Value      <- Number / String
```

And tagged structs:

``` go
type Assignment struct {
    Identifier string      `ll:"Identifier"`
    Value      AssignValue `ll:"Value"`
}

type AssignValue struct {
    Number *string `ll:"Number"`
    String *string `ll:"String"`
}
```

The generator emits arena-direct extraction:

``` go
func ExtractAssignment(t *tree, id NodeID) (Assignment, error) {
    var out Assignment
    n := &t.nodes[id]
    // child 0: Identifier (leaf, extract as string)
    // child 1: '=' literal -- dead child, skipped
    // child 2: Value (choice)
    // ...
}
```

Cross-validation at `go generate` time catches mismatches: unknown rules, arity
mismatches, type/kind conflicts.

## Limitations

- `TreeExtractor` interface and `Extract*` functions expose `*tree`, limiting
  extraction to the same package as the generated parser.
- Grammar-native struct generation (`langlang types`) is deferred; users write
  structs and tag them manually.
- Encode path (struct to tree to text) is deferred.

## More Information

- Full design and 12-task implementation plan:
  [langlang-extract-plan.md](../references/langlang-extract-plan.md)
- Analogous prior art: tommy (`github.com/amarbel-llc/tommy`) for TOML CST
  extraction
- Related: [FDR-0002 Typed Arena Strategies](0002-typed-arena-strategies.md)
  (arena allocation for extracted nodes)
