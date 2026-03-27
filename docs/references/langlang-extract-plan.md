# Langlang Extract: Static Tree Extraction via go:generate

## Problem

Langlang parsers produce a `Tree` interface backed by an arena-allocated `tree`
struct. Consumers that want typed Go data from a parse result must hand-write
tree-walking code: checking `Name()` against strings, navigating `Children()` by
index, handling optionals, and converting `Text()` to typed values. This
boilerplate is proportional to grammar complexity, error-prone (positional
indexing is fragile), and pays for interface dispatch on every node access even
though the grammar structure is statically known.

Tommy (github.com/amarbel-llc/tommy) solved an analogous problem for TOML: a
`//go:generate` directive inspects Go structs with `toml:` tags, then emits
specialized decode/encode code that operates directly on the CST (Concrete
Syntax Tree) without runtime reflection. The same approach applies here, with
the added advantage that the PEG grammar is available at generation time,
enabling static validation and arena-direct access that tommy cannot achieve.

## Solution

A `//go:generate langlang extract` directive that reads both the Go struct
definitions (via `go/ast` + `go/types`) and the PEG grammar (via langlang's
`QueryAST` + `QueryBytecode`), then emits extraction functions that index
directly into the tree's arena slices. No interface dispatch, no string
comparisons for rule names, no positional guesswork.

## Architecture

```
┌────────────────────────────────────────────────┐
│  Consumer code                                 │
│                                                │
│  //go:generate langlang extract                │
│       -grammar=json.peg                        │
│                                                │
│  type JSONValue struct {                       │
│      Object *JSONObject `ll:"Object"`          │
│      String *string     `ll:"String"`          │
│  }                                             │
├────────────────────────────────────────────────┤
│  extract/                                      │
│    analyze.go    — struct analysis             │  go/ast + go/types → FieldInfo
│    grammar.go    — grammar analysis            │  QueryAST + QueryBytecode → RuleInfo
│    validate.go   — cross-validation            │  struct fields vs grammar rules
│    emit.go       — per-field codegen           │  arena-direct tree walking
│    template.go   — file skeleton               │  text/template for signatures
│    generate.go   — orchestrator                │  wires everything together
├────────────────────────────────────────────────┤
│  langlang gen.go (existing)                    │
│    Embeds tree.go, vm.go, etc. into consumer   │  tree/node structs are local
│    Generated parser returns *tree (new)         │  concrete type, not Tree interface
└────────────────────────────────────────────────┘
```

## Key Insight: Arena-Direct Access

Langlang's `GenGoEval` already embeds the `tree` and `node` structs into the
consumer's package (via `//go:embed`). The unexported types become
package-local. Generated extraction code in the same package can access the
arena internals directly:

```go
// Interface path (current, per node):
//   Tree.Name(id)  →  interface dispatch → t.strs[t.nodes[id].nameID]
//   Tree.Children(id) → interface dispatch → type switch → childRanges → slice

// Generated path (what we emit):
//   n := &t.nodes[id]
//   cr := t.childRanges[n.childID]
//   kids := t.children[cr.start:cr.end]
```

No interface dispatch, no type switch, no `Name()` string comparison. Rule names
resolve to integer `nameID` constants at generation time from the bytecode
string table.

## Grammar-Aware Field Classification

Tommy classifies struct fields by Go type alone (it doesn't have access to the
TOML spec at codegen time). Langlang extract classifies fields by **both** Go
type and grammar structure. The generator loads the `.peg` file via `QueryAST`,
walks `GrammarNode.DefsByName` to find each rule referenced by a struct tag, and
inspects the rule's expression AST to determine the extraction strategy.

### FieldKind Taxonomy

```go
type FieldKind int

const (
    // Terminal extraction: rule body is a leaf pattern (literal, charset,
    // range, etc.). Field type must be string or a TextUnmarshaler.
    FieldText FieldKind = iota

    // Named rule reference: rule body is or contains an IdentifierNode
    // pointing to another rule. Field type must be a struct with its own
    // ll: tags, or *struct for optional rules.
    FieldNamedRule

    // Optional wrapper: grammar uses OptionalNode (?). Field must be a
    // pointer type. Inner classification recurses.
    FieldOptional

    // Repetition: grammar uses ZeroOrMoreNode (*) or OneOrMoreNode (+).
    // Field must be a slice. Inner classification recurses.
    FieldSlice

    // Ordered choice: grammar uses ChoiceNode (/). Field must be a struct
    // where each sub-field is a pointer corresponding to one alternative.
    FieldChoice

    // Sequence position: field maps to a specific child index within a
    // SequenceNode, determined by matching the child's rule name or
    // capture name against the field's tag.
    FieldSequenceChild

    // Capture: grammar uses explicit CaptureNode (#name{...}). Field
    // tag matches the capture name rather than a rule name.
    FieldCapture

    // Custom: type implements an Extractor interface
    // (UnmarshalTree(Tree, NodeID) error). Generator delegates to it.
    FieldCustom
)
```

### Grammar-to-Field Mapping

For each struct field tagged `ll:"RuleName"`, the generator looks up
`GrammarNode.DefsByName["RuleName"]` and inspects `DefinitionNode.Expr`:

| Grammar expression          | Go AST node type    | Resulting FieldKind    |
|-----------------------------|---------------------|------------------------|
| `[0-9]+`                    | OneOrMoreNode/Range | FieldText              |
| `'literal'`                 | LiteralNode         | FieldText              |
| `OtherRule`                 | IdentifierNode      | FieldNamedRule         |
| `OtherRule?`                | OptionalNode        | FieldOptional          |
| `OtherRule*` / `OtherRule+` | Zero/OneOrMoreNode  | FieldSlice             |
| `A / B / C`                 | ChoiceNode (nested) | FieldChoice            |
| `A '=' B`                   | SequenceNode        | (parent; children map) |
| `#name{ expr }`             | CaptureNode         | FieldCapture           |

## Cross-Validation at Generation Time

Unlike tommy, the generator has the grammar AST. This enables hard errors at
`go generate` time for mismatches:

1. **Unknown rule**: `ll:"Foo"` but "Foo" is not in `DefsByName` → error
2. **Arity mismatch**: grammar rule is a 3-alternative choice but struct has 2
   pointer fields → error
3. **Type mismatch**: grammar rule is a repetition (`*`/`+`) but field is not a
   slice → error
4. **Missing fields**: sequence has 3 captured children but struct only maps 2 →
   warning (unmatched nodes reported via `Unmatched()`)

This is a static correctness guarantee that tommy cannot offer because the TOML
spec is implicit.

## Concrete Type Return from Generated Parser

Currently `GenGoEval` generates parser methods that return `(Tree, error)` — the
interface. For arena-direct access, the extraction code needs `*tree`. Two
approaches, in order of preference:

1. **Add a concrete return method**: The generated parser gains a
   `ParseRaw() (*tree, error)` method alongside the existing `Parse() (Tree,
   error)`. This is a non-breaking addition. Consumers who use `Tree` are
   unaffected. Extract code calls `ParseRaw()`.

2. **Type assertion**: Generated extract code does `t := tree.(*tree)` inside
   the decode function. Works because both parser and extractor are in the same
   package after embedding. Uglier, but requires zero changes to langlang's
   existing codegen.

Option 1 is preferred. The change is a one-line addition to
`writeParserMethods` in `gen.go`.

## Static Optimizations

Three optimizations that become possible when the grammar structure is known at
codegen time:

### Constant-Folded Name Checks

The bytecode string table (`Bytecode.smap`) maps rule names to integer IDs. The
generator resolves `ll:"Assignment"` to its `nameID` at generation time and
emits `if n.nameID == 7` instead of `if tree.Name(id) == "Assignment"`. The IDs
are stable for a given grammar compilation.

### Dead Child Elimination

For a sequence like `Identifier '=' Value`, the literal `'='` produces a
`NodeType_String` in the tree but is never extracted. The generator inspects the
`SequenceNode.Items` in the grammar AST: items that are `LiteralNode`,
`CharsetNode`, or `RangeNode` without a `CaptureNode` wrapper are structural
punctuation. The generated code skips them entirely when iterating children.

### Flattened Single-Child Indirection

When a named rule wraps a single child (`Rule <- InnerRule`), the tree adds a
`NodeType_Node` with one child. The generator can see this in the grammar and
collapse the indirection, going directly to the inner node's data. This mirrors
tommy's `InnerInfo *StructInfo` inlining for nested structs.

## Generated Code Shape

For a grammar:

```peg
Assignment <- Identifier '=' Value
Identifier <- [a-zA-Z]+
Value      <- Number / String
Number     <- [0-9]+
String     <- '"' (!'"' .)* '"'
```

And a Go struct:

```go
//go:generate langlang extract -grammar=expr.peg
type Assignment struct {
    Identifier string      `ll:"Identifier"`
    Value      AssignValue `ll:"Value"`
}

type AssignValue struct {
    Number *string `ll:"Number"`
    String *string `ll:"String"`
}
```

The generator produces `assignment_extract.go`:

```go
// Code generated by langlang extract; DO NOT EDIT.
// Grammar: expr.peg

package expr

// Name ID constants (from bytecode string table)
const (
    _nameID_Assignment int32 = 0
    _nameID_Identifier int32 = 1
    _nameID_Value      int32 = 2
    _nameID_Number     int32 = 3
    _nameID_String     int32 = 4
)

func ExtractAssignment(t *tree, id NodeID) (Assignment, error) {
    var out Assignment
    n := &t.nodes[id]

    // Assignment is a sequence: Identifier '=' Value
    // Grammar analysis: child 0 = Identifier (named), child 1 = '=' (literal, skip), child 2 = Value (named)
    seq := &t.nodes[n.childID]
    cr := t.childRanges[seq.childID]
    kids := t.children[cr.start:cr.end]

    // child 0: Identifier → FieldText (leaf rule, extract as string)
    {
        inner := &t.nodes[kids[0]]
        leaf := &t.nodes[inner.childID]
        out.Identifier = string(t.input[leaf.start:leaf.end])
    }

    // child 1: '=' literal — dead child, skipped

    // child 2: Value → FieldChoice (Number / String)
    {
        var err error
        out.Value, err = extractAssignValue(t, kids[2])
        if err != nil {
            return out, err
        }
    }

    return out, nil
}

func extractAssignValue(t *tree, id NodeID) (AssignValue, error) {
    var out AssignValue
    n := &t.nodes[id]

    // Value is a choice: Number / String
    // The tree contains whichever alternative matched, as a named child.
    child := &t.nodes[n.childID]
    switch child.nameID {
    case _nameID_Number:
        leaf := &t.nodes[child.childID]
        s := string(t.input[leaf.start:leaf.end])
        out.Number = &s
    case _nameID_String:
        leaf := &t.nodes[child.childID]
        s := string(t.input[leaf.start:leaf.end])
        out.String = &s
    }

    return out, nil
}
```

## Extractor Interface (Custom Types)

For types that need custom extraction logic (analogous to tommy's
`TOMLUnmarshaler`):

```go
type TreeExtractor interface {
    ExtractFromTree(t *tree, id NodeID) error
}
```

The generator checks field types against this interface using `go/types`. If a
field implements `TreeExtractor`, the generated code delegates:

```go
if err := out.MyField.ExtractFromTree(t, kids[2]); err != nil {
    return out, fmt.Errorf("MyField: %w", err)
}
```

## Unmatched Node Tracking

Tommy tracks consumed TOML keys via a `consumed map[string]bool` and exposes
`Undecoded() []string`. The equivalent for langlang extract:

```go
type AssignmentExtraction struct {
    data    Assignment
    visited []bool // indexed by NodeID; same length as t.nodes
}

func (e *AssignmentExtraction) Data() *Assignment { return &e.data }

func (e *AssignmentExtraction) Unmatched(t *tree) []NodeID {
    var unmatched []NodeID
    for i, v := range e.visited {
        if !v && t.nodes[i].typ == NodeType_Node {
            unmatched = append(unmatched, NodeID(i))
        }
    }
    return unmatched
}
```

This catches grammar/struct drift: if a rule gains a new child, the unmatched
list surfaces it without breaking existing extraction.

## Project Structure (within langlang fork)

```
go/
├── extract/                     # new package
│   ├── analyze.go               # go/ast + go/types → StructInfo
│   ├── analyze_test.go
│   ├── grammar.go               # QueryAST + QueryBytecode → RuleInfo
│   ├── grammar_test.go
│   ├── validate.go              # cross-validation
│   ├── validate_test.go
│   ├── emit.go                  # per-field codegen (fmt.Fprintf)
│   ├── emit_test.go
│   ├── template.go              # text/template for file skeleton
│   ├── template_test.go
│   ├── generate.go              # orchestrator
│   ├── generate_test.go
│   └── integration_test.go      # end-to-end: grammar + struct → generated code → test
├── cmd/
│   └── langlang/
│       ├── main.go              # existing
│       └── extract.go           # new subcommand
├── gen.go                       # modified: add ParseRaw() to generated parsers
└── api.go                       # modified: add TreeExtractor interface
```

## Adoption Path

1. **Langlang fork ships extract alongside existing codegen** — `langlang
   generate` (parser codegen) is unchanged. `langlang extract` is a new
   subcommand.
2. **Dodder adopts per-format** — the organize text reader (user-facing,
   complex) is the first candidate. The box format reader (internal, simpler) can
   follow later.
3. **Future: grammar-native struct generation** — instead of the user writing
   structs and tagging them, a `langlang types` subcommand generates the struct
   definitions directly from the grammar. This inverts the dependency: grammar is
   source of truth, structs are derived. Deferred because manual struct
   definition gives the consumer control over which parts of the tree they
   extract.

## Rollback

Purely additive. Delete `_extract.go` files and remove `//go:generate langlang
extract` directives. The `Tree` interface path remains fully functional. No
existing consumer is affected.

## Deferred

- `langlang types` (generate structs from grammar)
- Incremental re-extraction (reuse extraction result after partial re-parse)
- Encode path (structured data → tree → text, the inverse of extraction)
- LSP integration for `ll:` tag validation and completion
- Benchmarking harness comparing interface-path vs arena-direct extraction

---

## Implementation Plan

**Goal:** Emit compile-time extraction functions that walk langlang parse trees
via direct arena access, driven by `//go:generate langlang extract` directives
on Go structs tagged with PEG rule names.

**Tech Stack:** `go/ast`, `go/types`, `golang.org/x/tools/go/packages`,
`text/template`, `fmt.Fprintf`, langlang's `QueryAST` / `QueryBytecode`

**Rollback:** Delete generated `_extract.go` files and remove directives. All
existing interfaces remain functional.

---

### Task 1: Add ParseRaw() to generated parsers

The extract codegen needs `*tree`, not the `Tree` interface. Add a concrete
return method to the generated parser.

**Files:**

- Modify: `go/gen.go`
- Test: `go/gen_test.go` or manual verification via `go/examples/`

**Implementation:**

In `writeParserMethods`, after the line that emits `Parse() (Tree, error)`, add:

```go
g.parser.writel(fmt.Sprintf(
    "func (p *%s) ParseRaw() (*tree, int, error) { return p.parseFnRaw(5) }",
    g.options.ParserName,
))
g.parser.writel(fmt.Sprintf(
    "func (p *%s) parseFnRaw(addr int) (*tree, int, error) {",
    g.options.ParserName,
))
g.parser.indent()
g.parser.writeil("val, n, err := p.vm.MatchRule(p.input, addr)")
g.parser.writeil("if err != nil { return nil, n, err }")
g.parser.writeil("return val.(*tree), n, nil")
g.parser.unindent()
g.parser.writel("}")
```

Also add per-rule `Parse<RuleName>Raw` methods using the same pattern as the
existing `Parse<RuleName>` loop.

**Verification:** Regenerate an example parser (e.g., JSON), confirm it compiles
and `ParseRaw()` returns `*tree` with the same data as `Parse()`.

---

### Task 2: Define the TreeExtractor interface

**Files:**

- Modify: `go/api.go`
- Test: `go/api_test.go`

**Implementation:**

Append to `api.go`:

```go
// TreeExtractor is implemented by types that perform custom extraction
// from a parse tree node. The generator delegates to this method when a
// struct field's type is not a recognized primitive or tagged struct.
//
// The tree and NodeID passed are arena-internal types. This interface is
// only usable within the same package as the generated parser (where
// the tree type is defined).
type TreeExtractor interface {
    ExtractFromTree(t *tree, id NodeID) error
}
```

Note: this interface references `*tree` (unexported). It can only be satisfied
within the generated package, which is exactly the scope where extraction code
lives. Document this constraint.

**Verification:** Write a test type implementing the interface; confirm it
compiles when in the same package as the tree definition.

---

### Task 3: Struct analysis (analyze.go)

Port tommy's analysis phase. The struct analysis is nearly identical: load the
package via `go/packages`, find structs annotated with `//go:generate langlang
extract`, read `ll:` tags instead of `toml:` tags, classify fields.

**Files:**

- Create: `go/extract/analyze.go`
- Create: `go/extract/analyze_test.go`

**Implementation:**

Reuse tommy's patterns directly:

- `StructInfo` and `FieldInfo` types, with `FieldKind` replaced by the
  PEG-oriented taxonomy (FieldText, FieldNamedRule, FieldOptional, FieldSlice,
  FieldChoice, FieldSequenceChild, FieldCapture, FieldCustom).
- `Analyze(dir, filename string) ([]StructInfo, error)` with the same
  `go/packages` loading and `hasGenerateDirective` check, using
  `"//go:generate langlang extract"` as the directive string.
- `extractLLTag(raw string) (string, tagOpts)` replacing `extractTomlTag`.
- Tag format: `` `ll:"RuleName"` `` with optional modifiers `` `ll:"RuleName,text"` ``
  to force FieldText extraction even when the rule has internal structure.
- `classifyField` initially classifies by Go type only. Grammar-aware
  reclassification happens in the validate phase (Task 5) after the grammar is
  loaded.

Initial Go-type-only classification rules:

| Go type                               | Initial FieldKind |
|----------------------------------------|-------------------|
| `string`                               | FieldText         |
| `*string`                              | FieldOptional     |
| `[]T` where T is string               | FieldSlice        |
| `*SomeStruct` with `ll:` tags         | FieldOptional     |
| `SomeStruct` with `ll:` tags          | FieldNamedRule    |
| `[]SomeStruct` with `ll:` tags        | FieldSlice        |
| implements `TreeExtractor`            | FieldCustom       |
| struct with all-pointer `ll:` fields  | FieldChoice       |

**Tests:** Unit tests for tag extraction and field classification using inline
Go source parsed by `go/packages`.

---

### Task 4: Grammar analysis (grammar.go)

Load the PEG grammar at generation time and build a `RuleInfo` map describing
each rule's structure.

**Files:**

- Create: `go/extract/grammar.go`
- Create: `go/extract/grammar_test.go`

**Implementation:**

```go
type RuleKind int

const (
    RuleLeaf     RuleKind = iota // body is terminal (literal, charset, range, any)
    RuleSequence                 // body is SequenceNode
    RuleChoice                   // body is ChoiceNode (possibly nested)
    RuleRepeat                   // body is ZeroOrMore or OneOrMore
    RuleOptional                 // body is OptionalNode
    RuleAlias                    // body is a single IdentifierNode (rule reference)
    RuleCapture                  // body is or contains CaptureNode
)

type RuleInfo struct {
    Name       string
    Kind       RuleKind
    NameID     int32          // from Bytecode.smap
    Children   []RuleChild    // for sequences: ordered children with metadata
    Choices    []string       // for choices: rule names of alternatives
    InnerRule  string         // for alias/optional/repeat: the referenced rule
    Definition *DefinitionNode // pointer back to grammar AST
}

type RuleChild struct {
    RuleName   string // rule or capture name, empty for literals
    IsLiteral  bool   // true for structural punctuation (dead child)
    IsCapture  bool   // true for explicit #name{} captures
    Index      int    // position in the SequenceNode.Items
}
```

The function `AnalyzeGrammar(grammarPath string) (map[string]RuleInfo, *Bytecode, error)`
does the following:

1. Create a `Database` with `NewRelativeImportLoader()`
2. Call `QueryAST(db, grammarPath)` → `*GrammarNode`
3. Call `QueryBytecode(db, grammarPath)` → `*Bytecode`
4. Walk `GrammarNode.Definitions`, classify each `DefinitionNode.Expr` by its
   AST node type
5. For sequences, enumerate `SequenceNode.Items` and tag each as literal vs
   rule-reference vs capture
6. Resolve `nameID` for each rule via `Bytecode.smap[ruleName]`

**Tests:** Load the JSON grammar (`grammars/json.peg`), verify RuleInfo for
Object, Array, Value, String, Number.

---

### Task 5: Cross-validation (validate.go)

Merge struct analysis and grammar analysis. Reclassify fields using grammar
knowledge. Emit hard errors for mismatches.

**Files:**

- Create: `go/extract/validate.go`
- Create: `go/extract/validate_test.go`

**Implementation:**

```go
func Validate(structs []StructInfo, rules map[string]RuleInfo) ([]StructInfo, []error)
```

For each struct field:

1. Look up `field.LLTag` in the rules map. Error if not found.
2. Compare `field.Kind` (from Go-type analysis) against `rule.Kind` (from
   grammar analysis). The valid combinations:
   - FieldText + RuleLeaf → OK
   - FieldText + RuleAlias where inner rule is leaf → OK (collapse)
   - FieldNamedRule + RuleSequence/RuleChoice/RuleAlias → OK
   - FieldOptional + RuleOptional → OK
   - FieldOptional + any rule (pointer field for choice branch) → OK
   - FieldSlice + RuleRepeat → OK
   - FieldChoice + RuleChoice → verify one pointer field per alternative
   - Anything else → error with explanation
3. For sequences: match struct fields to `RuleChild` entries. Each tagged
   field must correspond to exactly one non-literal child. Unmatched children
   go into the `Unmatched` set.
4. Annotate each `FieldInfo` with its resolved `nameID` and child index (for
   sequence fields).

**Tests:** Deliberate mismatch cases: wrong arity, missing rule, type/kind
conflict.

---

### Task 6: Code emission (emit.go)

Generate arena-direct tree-walking code for each field kind. This is the
core of the system and the direct analogue of tommy's `emit.go`.

**Files:**

- Create: `go/extract/emit.go`
- Create: `go/extract/emit_test.go`

**Implementation:**

Port tommy's `emitDecodeField` pattern (switch on FieldKind, emit via
`fmt.Fprintf`), but targeting tree arena access instead of document API calls.

```go
func emitExtractField(fi FieldInfo, treePath, targetPath string) string
```

Key emission patterns per FieldKind:

**FieldText:**
```go
// Arena-direct: no Name() call, no Text() call
{
    inner := &t.nodes[kids[%d]]
    leaf := &t.nodes[inner.childID]
    %s = string(t.input[leaf.start:leaf.end])
}
```

**FieldNamedRule:**
```go
{
    var err error
    %s, err = extract%s(t, kids[%d])
    if err != nil {
        return out, fmt.Errorf("%s: %%w", err)
    }
}
```

**FieldOptional:**
```go
if int(n.childID) >= 0 {
    child := &t.nodes[n.childID]
    if child.nameID == %d {
        val, err := extract%s(t, NodeID(n.childID))
        if err != nil {
            return out, fmt.Errorf("%s: %%w", err)
        }
        %s = &val
    }
}
```

**FieldSlice:**
```go
{
    cr := t.childRanges[seq.childID]
    kids := t.children[cr.start:cr.end]
    %s = make([]%s, 0, len(kids))
    for _, kid := range kids {
        child := &t.nodes[kid]
        if child.nameID == %d {
            val, err := extract%s(t, kid)
            if err != nil {
                return out, fmt.Errorf("%s[%%d]: %%w", len(%s), err)
            }
            %s = append(%s, val)
        }
    }
}
```

**FieldChoice:**
```go
{
    child := &t.nodes[n.childID]
    switch child.nameID {
    case %d: // RuleName1
        ...
        %s = &val
    case %d: // RuleName2
        ...
        %s = &val
    }
}
```

**FieldCustom:**
```go
if err := %s.ExtractFromTree(t, kids[%d]); err != nil {
    return out, fmt.Errorf("%s: %%w", err)
}
```

**Tests:** Golden-file tests comparing emitted code strings against expected
output for each FieldKind.

---

### Task 7: File template (template.go)

The `text/template` skeleton that wraps the emitted field code.

**Files:**

- Create: `go/extract/template.go`
- Create: `go/extract/template_test.go`

**Implementation:**

```go
const fileTemplate = `// Code generated by langlang extract; DO NOT EDIT.
// Grammar: {{.GrammarPath}}

package {{.Package}}

import "fmt"

// Ensure import is used.
var _ = fmt.Errorf

// Name ID constants (from bytecode string table)
const (
{{- range .NameIDs}}
    _nameID_{{.Name}} int32 = {{.ID}}
{{- end}}
)
{{range .Structs}}
func Extract{{.Name}}(t *tree, id NodeID) ({{.Name}}, error) {
    var out {{.Name}}
    n := &t.nodes[id]

{{emitExtract .}}
    return out, nil
}

func Extract{{.Name}}FromRoot(t *tree) ({{.Name}}, error) {
    root, ok := t.Root()
    if !ok {
        return {{.Name}}{}, fmt.Errorf("empty tree")
    }
    return Extract{{.Name}}(t, root)
}
{{end}}`
```

**Tests:** Render the template with mock StructInfo, verify output compiles.

---

### Task 8: Orchestrator (generate.go)

Wire the phases together.

**Files:**

- Create: `go/extract/generate.go`
- Create: `go/extract/generate_test.go`

**Implementation:**

```go
func Generate(dir, filename, grammarPath string) error {
    // Phase 1: Analyze Go structs
    structs, err := Analyze(dir, filename)

    // Phase 2: Analyze grammar
    rules, bytecode, err := AnalyzeGrammar(grammarPath)

    // Phase 3: Cross-validate
    structs, errs := Validate(structs, rules)

    // Phase 4: Render
    // ... (same pattern as tommy: RenderFile → format.Source → WriteFile)

    outName := strings.TrimSuffix(filename, ".go") + "_extract.go"
    // ...
}
```

Output file naming: `<source>_extract.go` (parallel to tommy's
`<source>_tommy.go`).

---

### Task 9: CLI subcommand

**Files:**

- Create: `go/cmd/langlang/extract.go`

**Implementation:**

Invoked as `langlang extract -grammar=path/to/grammar.peg`. Reads `$GOFILE`
from the environment (set by `go generate`). Calls
`extract.Generate(dir, filename, grammarPath)`.

Usage in source files:

```go
//go:generate langlang extract -grammar=../../grammars/json.peg
type JSONValue struct {
    // ...
}
```

---

### Task 10: Integration test — JSON grammar

End-to-end test using `grammars/json.peg`.

**Files:**

- Create: `go/extract/integration_test.go`

**Implementation:**

1. Write a temp Go file with structs tagged against JSON grammar rules
2. Run `Generate(dir, "json_types.go", jsonGrammarPath)`
3. Write a test file that parses JSON input via the generated parser, then
   calls `ExtractJSONValue` on the result
4. Run `go test` in the temp directory
5. Verify extracted Go values match expected data

Test inputs: `{"name": "test", "count": 42}`, `[1, 2, 3]`, nested objects.

---

### Task 11: Integration test — custom TreeExtractor

Verify that types implementing `TreeExtractor` are handled correctly.

**Files:**

- Modify: `go/extract/integration_test.go`

Same pattern as Task 10 but with a custom type that implements
`ExtractFromTree`.

---

### Task 12: Full test suite, no regressions

Run langlang's existing test suite to confirm nothing is broken.

```
go test ./go/...
```

All existing parser generation, VM, query, and LSP tests must pass. The
`gen.go` change (Task 1) adds methods but doesn't alter existing ones.

---

## Open Questions

1. **Tag syntax**: `ll:"RuleName"` is clean but collides with nothing today.
   Alternative: `peg:"RuleName"` if the fork diverges from langlang's name.
   Decide before Task 3.

2. **Sequence child mapping strategy**: Two options for how struct fields map
   to sequence children. Option A: by rule name (field tag matches child's rule
   name, position is inferred). Option B: by explicit index
   (`ll:"Assignment.2"` or `ll:"Value,index=2"`). Option A is less fragile;
   option B handles sequences with repeated rule references (e.g.,
   `Expr Operator Expr` where both operands have the same rule name). Could
   support both, with name-based as default and index as escape hatch.

3. **`*tree` exposure**: The `TreeExtractor` interface and `Extract*` functions
   expose `*tree` in the consumer package's API surface. This is fine when the
   consumer is the same package as the generated parser (the intended case), but
   it prevents extraction from a separate package. If cross-package extraction
   is needed later, an exported `RawTree` type alias or a thin wrapper would
   be required.

---

## Architectural Note: Direct Codegen Without the VM

This plan assumes the existing architecture: the grammar compiles to bytecode,
`GenGoEval` embeds the VM interpreter into the consumer's package, the VM
produces a `tree`, and the extraction codegen walks the tree. The VM is the
parser; extraction is a separate post-parse step.

A more aggressive approach eliminates the VM entirely for the ahead-of-time
codegen path. Instead of compiling the grammar to bytecode and interpreting it,
the codegen emits native Go functions directly from the grammar AST
(`GrammarNode`). Each rule becomes a function. Ordered choice becomes a
sequence of `if` branches. Repetition becomes a `for` loop. Byte matching
becomes `input[pos] != 'x'`. The Go compiler optimizes the result — direct
calls, inlined comparisons, hardware branch prediction on the actual control
flow instead of the VM's generic dispatch loop.

In this model, extraction doesn't walk a tree after parsing. Instead, the
generated parse functions return typed structs directly — parsing and
extraction are fused into a single pass. The `tree` / `node` arena, the
`Tree` interface, and Tasks 1-2 (adding `ParseRaw`, defining `TreeExtractor`)
become unnecessary. Tasks 3-5 (struct analysis, grammar analysis,
cross-validation) are unchanged — you still need to map struct fields to
grammar rules. Tasks 6-8 (emit, template, orchestrator) change their target:
instead of emitting tree-walking extraction code, they emit fused
parse-and-extract functions.

The codegen chain shortens from:

```
grammar → AST → compiler → Program → encoder → Bytecode → GenGoEval (VM embed)
                                                           ↓
                                                     extraction codegen (tree walk)
```

to:

```
grammar → AST → transform → direct emitter (one function per rule, typed output)
```

The VM interpreter remains available for runtime grammar compilation
(`MatcherFromString`, REPL, dynamic grammars), but the ahead-of-time path
bypasses it. The VM's instruction set serves as a design vocabulary — a
catalog of PEG patterns to emit — not a runtime to execute.
