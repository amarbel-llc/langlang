# Binary Codegen MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add binary-format grammar primitives and Go struct codegen to
langlang, enabling grammar-driven encode/decode for dodder's WASM wire format.

**Architecture:** Extend `langlang.peg` with four new syntactic forms (name
binding, bytes consume, counted repetition, numeric primitive). Add
corresponding AST nodes. New `go/binary/` package walks the AST and emits
standalone Go structs + Decode/Encode/Size functions. New `langlang codegen` CLI
subcommand.

**Tech Stack:** Go, PEG grammar, `encoding/binary` in generated code.

**Rollback:** Purely additive. Revert commits to remove.

--------------------------------------------------------------------------------

### Task 1: Extend langlang.peg with binary syntax

**Files:** - Modify: `grammars/langlang.peg`

**Step 1: Add new productions to langlang.peg**

The grammar currently has:

    Sequence <- Prefix*
    Suffix   <- Primary ("?" / "*" / "+" / Superscript)?
    Primary  <- Identifier !LEFTARROW / "(" Expression ")" / List / Literal / Class / Any

Add these changes:

``` peg
// Change Sequence to go through Binding
Sequence    <- Binding*
Binding     <- (Identifier ':')? Prefix

// Add CountedSuffix to Suffix alternatives
Suffix      <- Primary ("?" / "*" / "+" / Superscript / CountedSuffix)?
CountedSuffix <- "{" Identifier "}"

// Add BytesConsume before Identifier in Primary (must come first so "bytes(" takes priority)
Primary     <- BytesConsume
             / Identifier !LEFTARROW
             / "(" Expression ")"^MissingClosingParen
             / List / Literal / Class / Any
BytesConsume <- "bytes" "(" Identifier ")"
```

No grammar change for numeric primitives (`u32le` etc.) --- they parse as
`Identifier` and are converted to `NumericPrimitiveNode` during AST
construction.

**Step 2: Regenerate bootstrap parser**

Run: `just generate`

This rebuilds `build/langlang` from current code, then runs `go generate` which
regenerates `go/grammar_parser_bootstrap.go` from the updated `langlang.peg`.

**Step 3: Verify existing tests still pass**

Run: `just test` Expected: All existing tests pass (new grammar rules are
additive).

**Step 4: Commit**

    feat: extend langlang.peg with binding, bytes, and counted repetition syntax

--------------------------------------------------------------------------------

### Task 2: Add new AST node types

**Files:** - Modify: `go/grammar_ast.go`

**Step 1: Add NumericPrimitiveNode**

Add after the IdentifierNode section (around line 660):

``` go
// Node Type: NumericPrimitive

type NumericPrimitiveNode struct {
    src       SourceLocation
    Name      string // e.g. "u32le"
    Width     int    // bytes: 1, 2, 4, 8
    BigEndian bool
}

func NewNumericPrimitiveNode(name string, width int, bigEndian bool, s SourceLocation) *NumericPrimitiveNode {
    return &NumericPrimitiveNode{Name: name, Width: width, BigEndian: bigEndian, src: s}
}

func (n NumericPrimitiveNode) SourceLocation() SourceLocation { return n.src }
func (n NumericPrimitiveNode) String() string                 { return n.Name }
func (n NumericPrimitiveNode) PrettyString() string           { return ppAstNode(&n, formatNodePlain) }
func (n NumericPrimitiveNode) HighlightPrettyString() string  { return ppAstNode(&n, formatNodeThemed) }
func (n NumericPrimitiveNode) Accept(v AstNodeVisitor) error  { return v.VisitNumericPrimitiveNode(&n) }

func (n NumericPrimitiveNode) Equal(o AstNode) bool {
    other, ok := o.(*NumericPrimitiveNode)
    if !ok {
        return false
    }
    return n.Name == other.Name
}
```

**Step 2: Add NameBindingNode**

``` go
// Node Type: NameBinding

type NameBindingNode struct {
    src  SourceLocation
    Name string
    Expr AstNode
}

func NewNameBindingNode(name string, expr AstNode, s SourceLocation) *NameBindingNode {
    return &NameBindingNode{Name: name, Expr: expr, src: s}
}

func (n NameBindingNode) SourceLocation() SourceLocation { return n.src }
func (n NameBindingNode) String() string                 { return fmt.Sprintf("%s:%s", n.Name, n.Expr.String()) }
func (n NameBindingNode) PrettyString() string           { return ppAstNode(&n, formatNodePlain) }
func (n NameBindingNode) HighlightPrettyString() string  { return ppAstNode(&n, formatNodeThemed) }
func (n NameBindingNode) Accept(v AstNodeVisitor) error  { return v.VisitNameBindingNode(&n) }

func (n NameBindingNode) Equal(o AstNode) bool {
    other, ok := o.(*NameBindingNode)
    if !ok {
        return false
    }
    return n.Name == other.Name && n.Expr.Equal(other.Expr)
}
```

**Step 3: Add BytesConsumeNode**

``` go
// Node Type: BytesConsume

type BytesConsumeNode struct {
    src  SourceLocation
    Name string // references a NameBindingNode
}

func NewBytesConsumeNode(name string, s SourceLocation) *BytesConsumeNode {
    return &BytesConsumeNode{Name: name, src: s}
}

func (n BytesConsumeNode) SourceLocation() SourceLocation { return n.src }
func (n BytesConsumeNode) String() string                 { return fmt.Sprintf("bytes(%s)", n.Name) }
func (n BytesConsumeNode) PrettyString() string           { return ppAstNode(&n, formatNodePlain) }
func (n BytesConsumeNode) HighlightPrettyString() string  { return ppAstNode(&n, formatNodeThemed) }
func (n BytesConsumeNode) Accept(v AstNodeVisitor) error  { return v.VisitBytesConsumeNode(&n) }

func (n BytesConsumeNode) Equal(o AstNode) bool {
    other, ok := o.(*BytesConsumeNode)
    if !ok {
        return false
    }
    return n.Name == other.Name
}
```

**Step 4: Add CountedRepetitionNode**

``` go
// Node Type: CountedRepetition

type CountedRepetitionNode struct {
    src   SourceLocation
    Expr  AstNode
    Count string // references a NameBindingNode
}

func NewCountedRepetitionNode(expr AstNode, count string, s SourceLocation) *CountedRepetitionNode {
    return &CountedRepetitionNode{Expr: expr, Count: count, src: s}
}

func (n CountedRepetitionNode) SourceLocation() SourceLocation { return n.src }
func (n CountedRepetitionNode) String() string                 { return fmt.Sprintf("%s{%s}", n.Expr.String(), n.Count) }
func (n CountedRepetitionNode) PrettyString() string           { return ppAstNode(&n, formatNodePlain) }
func (n CountedRepetitionNode) HighlightPrettyString() string  { return ppAstNode(&n, formatNodeThemed) }
func (n CountedRepetitionNode) Accept(v AstNodeVisitor) error  { return v.VisitCountedRepetitionNode(&n) }

func (n CountedRepetitionNode) Equal(o AstNode) bool {
    other, ok := o.(*CountedRepetitionNode)
    if !ok {
        return false
    }
    return n.Count == other.Count && n.Expr.Equal(other.Expr)
}
```

**Step 5: Verify it compiles**

Run: `cd go && go build ./...` Expected: Compile errors from visitor/printer
(expected --- fixed in next task).

**Do NOT commit yet** --- visitor and printer need updating first.

--------------------------------------------------------------------------------

### Task 3: Update visitor, printer, and Inspect

**Files:** - Modify: `go/grammar_ast_visitor.go` - Modify:
`go/grammar_ast_printer.go` - Modify: `go/grammar_ast_visitor.go` (the `Inspect`
function)

**Step 1: Add visitor interface methods**

In `AstNodeVisitor` interface in `go/grammar_ast_visitor.go`, add:

``` go
VisitNumericPrimitiveNode(*NumericPrimitiveNode) error
VisitNameBindingNode(*NameBindingNode) error
VisitBytesConsumeNode(*BytesConsumeNode) error
VisitCountedRepetitionNode(*CountedRepetitionNode) error
```

**Step 2: Add Inspect cases**

In the `inspect` function's type switch, add:

``` go
case *NumericPrimitiveNode:
    // Leaf node, no children

case *NameBindingNode:
    inspect(n.Expr, f, visited)

case *BytesConsumeNode:
    // Leaf node (Name is a string reference, not a child node)

case *CountedRepetitionNode:
    inspect(n.Expr, f, visited)
```

Also add `*NumericPrimitiveNode` and `*BytesConsumeNode` to the existing leaf
node case if preferred:

``` go
case *AnyNode, *LiteralNode, *IdentifierNode, *RangeNode, *CharsetNode,
     *NumericPrimitiveNode, *BytesConsumeNode:
```

**Step 3: Add printer methods**

In `go/grammar_ast_printer.go`, add visitor methods to `grammarPrinter`:

``` go
func (gp *grammarPrinter) VisitNumericPrimitiveNode(n *NumericPrimitiveNode) error {
    gp.writeOperand(n.Name)
    gp.writeSpanl(n)
    return nil
}

func (gp *grammarPrinter) VisitNameBindingNode(n *NameBindingNode) error {
    gp.writeOperator(fmt.Sprintf("Binding[%s]", n.Name))
    gp.writeSpanl(n)
    gp.pwrite("└── ")
    gp.indent("    ")
    n.Expr.Accept(gp)
    gp.unindent()
    return nil
}

func (gp *grammarPrinter) VisitBytesConsumeNode(n *BytesConsumeNode) error {
    gp.writeOperand(fmt.Sprintf("Bytes(%s)", n.Name))
    gp.writeSpanl(n)
    return nil
}

func (gp *grammarPrinter) VisitCountedRepetitionNode(n *CountedRepetitionNode) error {
    gp.writeOperator(fmt.Sprintf("Counted{%s}", n.Count))
    gp.writeSpanl(n)
    gp.pwrite("└── ")
    gp.indent("    ")
    n.Expr.Accept(gp)
    gp.unindent()
    return nil
}
```

**Step 4: Fix all remaining compile errors**

Any other visitor implementations in the codebase (compiler, handlers, query
analysis) need stub methods for the new node types. Search for `AstNodeVisitor`
implementations and add stubs that return `nil` or
`fmt.Errorf("not implemented")`.

Run: `cd go && go build ./...` Expected: Compiles cleanly.

**Step 5: Commit**

    feat: add AST nodes for binary primitives (NumericPrimitive, NameBinding, BytesConsume, CountedRepetition)

--------------------------------------------------------------------------------

### Task 4: Wire parse tree to new AST nodes in grammar_parser_v2.go

**Files:** - Modify: `go/grammar_parser_v2.go`

**Step 1: Add numeric primitive lookup table**

``` go
var numericPrimitives = map[string]struct{ Width int; BigEndian bool }{
    "u8":    {1, false},
    "u16le": {2, false},
    "u16be": {2, true},
    "u32le": {4, false},
    "u32be": {4, true},
    "u64le": {8, false},
    "u64be": {8, true},
}
```

**Step 2: Handle NumericPrimitive in parsePrimary**

In `parsePrimary`, after the `case "Identifier":` block, check if the identifier
text matches a numeric primitive:

``` go
case "Identifier":
    ident, err := p.parseIdentifier(childID)
    if err != nil {
        return nil, err
    }
    if prim, ok := numericPrimitives[ident.Name]; ok {
        return NewNumericPrimitiveNode(ident.Name, prim.Width, prim.BigEndian, ident.SourceLocation()), nil
    }
    return ident, nil
```

**Step 3: Handle BytesConsume in parsePrimary**

Add a case for the `BytesConsume` parse tree node (from the updated grammar):

``` go
case "BytesConsume":
    return p.parseBytesConsume(childID)
```

Add the method:

``` go
func (p *GrammarParserV2) parseBytesConsume(id NodeID) (*BytesConsumeNode, error) {
    // BytesConsume <- "bytes" "(" Identifier ")"
    // Parse tree: sequence of "bytes", "(", Identifier, ")"
    // We need the Identifier child
    var name string
    p.tree.Visit(id, func(childID NodeID) bool {
        if p.tree.Type(childID) == NodeType_Node && p.tree.Name(childID) == "Identifier" {
            name = p.tree.Text(childID)
            return false
        }
        return true
    })
    if name == "" {
        return nil, errors.New("parseBytesConsume: no identifier found")
    }
    return NewBytesConsumeNode(name, p.sloc(id)), nil
}
```

**Step 4: Handle Binding in parseSequence**

The grammar now has `Sequence <- Binding*` instead of `Sequence <- Prefix*`.
Update `parseSequence` to iterate over `Binding` children:

``` go
// In parseSequence, change p.parsePrefix(expID) calls to p.parseBinding(expID)
```

Add the method:

``` go
func (p *GrammarParserV2) parseBinding(id NodeID) (AstNode, error) {
    // Binding <- (Identifier ':')? Prefix
    childID, ok := p.tree.Child(id)
    if !ok {
        return nil, errors.New("parseBinding: no child node found")
    }
    childType := p.tree.Type(childID)

    switch childType {
    case NodeType_Sequence:
        // Has binding name: Identifier ':' Prefix
        items := slices.Collect(p.tree.IterDirectChildren(childID))
        // items[0] is the optional (Identifier ':') group, items[1] is Prefix
        // But the optional may have matched, making this a 2-item sequence
        // where items[0] contains Identifier+colon and items[1] is Prefix
        name := ""
        var prefixID NodeID
        // Find the Identifier and Prefix within the sequence
        for _, itemID := range items {
            if p.tree.Type(itemID) == NodeType_Node && p.tree.Name(itemID) == "Prefix" {
                prefixID = itemID
            } else if p.tree.Type(itemID) == NodeType_Node && p.tree.Name(itemID) == "Identifier" {
                name = p.tree.Text(itemID)
            }
        }
        if name == "" {
            // No binding, just a prefix
            return p.parsePrefix(items[0])
        }
        expr, err := p.parsePrefix(prefixID)
        if err != nil {
            return nil, err
        }
        return NewNameBindingNode(name, expr, p.sloc(childID)), nil
    case NodeType_Node:
        // No binding, just a Prefix
        return p.parsePrefix(childID)
    default:
        return nil, fmt.Errorf("unknown node type for parseBinding: %v", childType)
    }
}
```

**Note:** The exact parse tree structure depends on what the bootstrap parser
produces for `Binding <- (Identifier ':')? Prefix`. Debug by parsing a test
grammar with `-grammar-ast` and inspecting the tree. Adjust the `parseBinding`
implementation to match the actual tree structure.

**Step 5: Handle CountedSuffix in parseSuffix**

In `parseSuffix`, add handling for the `CountedSuffix` case. When the suffix is
a `CountedSuffix` node (or a sequence containing `{`, Identifier, `}`):

``` go
// In the suffix switch, add:
case NodeType_Node:
    if p.tree.Name(items[1]) == "CountedSuffix" {
        countName := p.extractIdentifierFromCountedSuffix(items[1])
        return NewCountedRepetitionNode(primary, countName, p.sloc(childID)), nil
    }
```

Add helper:

``` go
func (p *GrammarParserV2) extractIdentifierFromCountedSuffix(id NodeID) string {
    var name string
    p.tree.Visit(id, func(childID NodeID) bool {
        if p.tree.Type(childID) == NodeType_Node && p.tree.Name(childID) == "Identifier" {
            name = p.tree.Text(childID)
            return false
        }
        return true
    })
    return name
}
```

**Step 6: Verify it compiles**

Run: `cd go && go build ./...` Expected: Compiles.

**Do NOT commit yet** --- tests come first.

--------------------------------------------------------------------------------

### Task 5: Write grammar parser tests for binary syntax

**Files:** - Modify: `go/grammar_parser_test.go`

**Step 1: Write test for name binding**

``` go
func TestParseBinaryBindingSyntax(t *testing.T) {
    for _, test := range []struct {
        Name           string
        Grammar        string
        ExpectedOutput string
    }{
        {
            Name:    "NameBinding",
            Grammar: "A <- len:u32le",
            // Expected: Definition -> Sequence -> Binding[len] -> u32le
        },
        {
            Name:    "BytesConsume",
            Grammar: "A <- bytes(len)",
            // Expected: Definition -> Sequence -> Bytes(len)
        },
        {
            Name:    "CountedRepetition",
            Grammar: "A <- B{count}",
            // Expected: Definition -> Sequence -> Counted{count} -> Identifier[B]
        },
        {
            Name:    "NumericPrimitive",
            Grammar: "A <- u32le",
            // Expected: Definition -> Sequence -> u32le
        },
        {
            Name:    "PrefixedString",
            Grammar: "prefixed_string <- len:u32le data:bytes(len)",
            // Expected: Two bindings in sequence
        },
        {
            Name:    "PrefixedStringList",
            Grammar: "prefixed_string_list <- count:u32le items:prefixed_string{count}",
        },
    } {
        t.Run(test.Name, func(t *testing.T) {
            parser := NewGrammarParser([]byte(test.Grammar))
            output, err := parser.Parse()
            require.NoError(t, err)
            // Use PrettyString to verify tree structure
            t.Log(output.PrettyString())
            // Assert no errors in grammar
            grammar := output.(*GrammarNode)
            assert.Empty(t, grammar.Errors)
        })
    }
}
```

**Step 2: Run tests to verify they pass**

Run: `just test`

Note: The exact `ExpectedOutput` strings depend on the PrettyString output of
the new nodes. First run with just `t.Log` assertions, capture the output, then
fill in exact expected strings. This is iterative.

Expected: Tests pass with correct AST structure.

**Step 3: Commit**

    feat: wire binary syntax from parse tree to AST nodes

    Adds parseBinding, parseBytesConsume, extractIdentifierFromCountedSuffix
    to grammar_parser_v2.go. Numeric primitives (u8, u16le, u32le, etc.)
    are recognized by name and converted from IdentifierNode to
    NumericPrimitiveNode during AST construction.

--------------------------------------------------------------------------------

### Task 6: Add test grammar file

**Files:** - Create: `grammars/sku_record.peg`

**Step 1: Write the grammar**

``` peg
@whitespace none

sku_record <- genre:prefixed_string
              object_id:prefixed_string
              type_id:prefixed_string
              tags:prefixed_string_list
              tags_implicit:prefixed_string_list
              blob_digest:prefixed_string
              description:prefixed_string

prefixed_string <- len:u32le data:bytes(len)
prefixed_string_list <- count:u32le items:prefixed_string{count}
```

**Step 2: Verify it parses**

Run: `./build/langlang -grammar grammars/sku_record.peg -grammar-ast` Expected:
Prints the AST with Binding, NumericPrimitive, BytesConsume, and
CountedRepetition nodes.

**Step 3: Commit**

    feat: add sku_record.peg grammar for dodder wire format

--------------------------------------------------------------------------------

### Task 7: Create binary codegen package --- struct generation

**Files:** - Create: `go/binary/gen.go` - Create: `go/binary/gen_test.go`

**Step 1: Write failing test for struct generation**

``` go
package binary

import (
    "testing"

    langlang "github.com/clarete/langlang/go"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func parseGrammar(t *testing.T, input string) *langlang.GrammarNode {
    t.Helper()
    parser := langlang.NewGrammarParser([]byte(input))
    ast, err := parser.Parse()
    require.NoError(t, err)
    grammar, ok := ast.(*langlang.GrammarNode)
    require.True(t, ok)
    require.Empty(t, grammar.Errors)
    return grammar
}

func TestGeneratePrefixedString(t *testing.T) {
    grammar := parseGrammar(t, `@whitespace none
prefixed_string <- len:u32le data:bytes(len)`)

    output, err := Generate(grammar, Options{PackageName: "wire"})
    require.NoError(t, err)

    // Should contain struct with Data field (len is a length field, not in struct)
    assert.Contains(t, output, "type PrefixedString struct")
    assert.Contains(t, output, "Data []byte")
    // Should contain Decode, Encode, Size functions
    assert.Contains(t, output, "func DecodePrefixedString(")
    assert.Contains(t, output, "func EncodePrefixedString(")
    assert.Contains(t, output, "func SizePrefixedString(")
}
```

**Step 2: Run test to verify it fails**

Run: `cd go && go test -v -run TestGeneratePrefixedString ./binary/` Expected:
FAIL --- `Generate` doesn't exist yet.

**Step 3: Write minimal Generate implementation**

The `Generate` function:

1.  Analyzes each `DefinitionNode` to extract fields from `NameBindingNode`s
2.  Determines which bindings are "length fields" (referenced by
    `BytesConsumeNode` or `CountedRepetitionNode`) --- these become local
    variables, not struct fields
3.  Generates struct type, Decode, Encode, Size for each rule

``` go
package binary

import (
    "fmt"
    "strings"

    langlang "github.com/clarete/langlang/go"
)

type Options struct {
    PackageName string
    SourceFile  string
}

func Generate(grammar *langlang.GrammarNode, opts Options) (string, error) {
    var buf strings.Builder

    fmt.Fprintf(&buf, "// Code generated by langlang codegen, DO NOT EDIT.\n\n")
    fmt.Fprintf(&buf, "package %s\n\n", opts.PackageName)
    fmt.Fprintf(&buf, "import (\n")
    fmt.Fprintf(&buf, "\t\"encoding/binary\"\n")
    fmt.Fprintf(&buf, "\t\"io\"\n")
    fmt.Fprintf(&buf, ")\n\n")

    for _, def := range grammar.Definitions {
        info, err := analyzeRule(def)
        if err != nil {
            return "", fmt.Errorf("rule %s: %w", def.Name, err)
        }
        if info == nil {
            continue // rule has no bindings, skip
        }
        emitStruct(&buf, info)
        emitDecode(&buf, info)
        emitEncode(&buf, info)
        emitSize(&buf, info)
    }

    return buf.String(), nil
}
```

Implement `analyzeRule` to walk the definition's expression (SequenceNode of
NameBindingNodes) and produce a `ruleInfo` describing fields and their types.
Implement `emitStruct`, `emitDecode`, `emitEncode`, `emitSize` to emit Go code.

**Step 4: Run test to verify it passes**

Run: `cd go && go test -v -run TestGeneratePrefixedString ./binary/` Expected:
PASS.

**Step 5: Commit**

    feat(binary): add Go struct codegen from binary grammar rules

--------------------------------------------------------------------------------

### Task 8: Binary codegen --- round-trip test

**Files:** - Create: `go/binary/roundtrip_test.go`

**Step 1: Write round-trip test**

This test generates code from the full `sku_record.peg` grammar, compiles it,
and runs encode/decode. Since we can't compile generated code in a unit test
easily, use a different approach: test the generated Decode/Encode functions
inline using `go/binary` package internals.

Actually, the better approach: write the test against the **generated output**
by verifying the generated Go code compiles and round-trips. Use a test fixture
approach:

``` go
func TestRoundTripPrefixedString(t *testing.T) {
    // Hand-encode a prefixed string: len=5, data="hello"
    data := []byte{
        0x05, 0x00, 0x00, 0x00, // u32le: 5
        'h', 'e', 'l', 'l', 'o', // "hello"
    }

    grammar := parseGrammar(t, `@whitespace none
prefixed_string <- len:u32le data:bytes(len)`)

    output, err := Generate(grammar, Options{PackageName: "wire"})
    require.NoError(t, err)

    // Verify the generated code contains expected decode logic
    assert.Contains(t, output, "binary.LittleEndian.Uint32")
    _ = data // data used for golden test below
}
```

For a true round-trip test, create an integration test that: 1. Generates code
to a temp file 2. Compiles it with `go build` 3. Runs a test binary

This is the `go/tests/` pattern. Create `go/tests/binary/` with a `go:generate`
directive.

**Step 2: Create integration test directory**

Create `go/tests/binary/`: - `go/tests/binary/binary_test.go` --- round-trip
tests - `go/tests/binary/generate.go` --- `//go:generate` directive

The `generate.go` file:

``` go
package binary

//go:generate go run ../../cmd/langlang codegen -grammar ../../../grammars/sku_record.peg -lang go -output-path ./wire.go -go-package binary
```

The `binary_test.go` file:

``` go
package binary

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRoundTripPrefixedString(t *testing.T) {
    original := PrefixedString{Data: []byte("hello")}
    encoded, err := EncodePrefixedString(&original)
    require.NoError(t, err)

    decoded, n, err := DecodePrefixedString(encoded)
    require.NoError(t, err)
    assert.Equal(t, len(encoded), n)
    assert.Equal(t, original.Data, decoded.Data)
}

func TestRoundTripSkuRecord(t *testing.T) {
    original := SkuRecord{
        Genre:        PrefixedString{Data: []byte("zettel")},
        ObjectId:     PrefixedString{Data: []byte("abc123")},
        TypeId:       PrefixedString{Data: []byte("typ")},
        Tags:         PrefixedStringList{Items: []PrefixedString{{Data: []byte("tag1")}, {Data: []byte("tag2")}}},
        TagsImplicit: PrefixedStringList{Items: nil},
        BlobDigest:   PrefixedString{Data: []byte("sha256:deadbeef")},
        Description:  PrefixedString{Data: []byte("a test record")},
    }
    encoded, err := EncodeSkuRecord(&original)
    require.NoError(t, err)

    decoded, n, err := DecodeSkuRecord(encoded)
    require.NoError(t, err)
    assert.Equal(t, len(encoded), n)
    assert.Equal(t, original, decoded)
}

func TestGoldenBytes(t *testing.T) {
    ps := PrefixedString{Data: []byte("hi")}
    encoded, err := EncodePrefixedString(&ps)
    require.NoError(t, err)

    // u32le(2) = 0x02 0x00 0x00 0x00, then "hi"
    expected := []byte{0x02, 0x00, 0x00, 0x00, 'h', 'i'}
    assert.Equal(t, expected, encoded)
}

func TestDecodeShortInput(t *testing.T) {
    _, _, err := DecodePrefixedString([]byte{0x05, 0x00})
    assert.Error(t, err) // too short for u32le
}

func TestEmptyList(t *testing.T) {
    original := PrefixedStringList{Items: nil}
    encoded, err := EncodePrefixedStringList(&original)
    require.NoError(t, err)

    decoded, _, err := DecodePrefixedStringList(encoded)
    require.NoError(t, err)
    assert.Equal(t, 0, len(decoded.Items))
}
```

**Step 3: Run `just generate` then `just test`**

Run: `just test` Expected: Integration tests pass.

**Step 4: Commit**

    test: add round-trip and golden bytes tests for binary codegen

--------------------------------------------------------------------------------

### Task 9: Add `langlang codegen` CLI subcommand

**Files:** - Create: `go/cmd/langlang/codegen.go` - Modify:
`go/cmd/langlang/main.go`

**Step 1: Create the codegen subcommand handler**

``` go
package main

import (
    "flag"
    "fmt"
    "os"

    langlang "github.com/clarete/langlang/go"
    "github.com/clarete/langlang/go/binary"
)

func runCodegen(args []string) {
    fs := flag.NewFlagSet("codegen", flag.ExitOnError)
    grammar := fs.String("grammar", "", "Path to the grammar file (required)")
    lang := fs.String("lang", "", "Output language: go (required)")
    outputPath := fs.String("output-path", "", "Path to the output file (required)")
    goPackage := fs.String("go-package", "wire", "Go package name")
    fs.Parse(args)

    if *grammar == "" || *lang == "" || *outputPath == "" {
        fmt.Fprintf(os.Stderr, "error: -grammar, -lang, and -output-path are required\n")
        os.Exit(1)
    }

    if *lang != "go" {
        fatal("codegen: unsupported language %q (only 'go' is supported)", *lang)
    }

    cfg := langlang.NewConfig()
    // Binary grammars should not inject builtins or whitespace handling
    cfg.SetBool("grammar.add_builtins", false)
    cfg.SetBool("grammar.handle_spaces", false)
    cfg.SetBool("grammar.captures", false)

    loader := langlang.NewRelativeImportLoader()
    db := langlang.NewDatabase(cfg, loader)

    ast, err := langlang.QueryAST(db, *grammar)
    if err != nil {
        fatal("Failed to parse grammar: %s", err.Error())
    }

    output, err := binary.Generate(ast, binary.Options{
        PackageName: *goPackage,
        SourceFile:  *grammar,
    })
    if err != nil {
        fatal("codegen failed: %s", err.Error())
    }

    if err := os.WriteFile(*outputPath, []byte(output), 0644); err != nil {
        fatal("Can't write output: %s", err.Error())
    }
}
```

**Step 2: Wire into main.go subcommand dispatch**

In `main.go`, add the `codegen` case:

``` go
switch os.Args[1] {
case "extract":
    runExtract(os.Args[2:])
    return
case "codegen":
    runCodegen(os.Args[2:])
    return
}
```

**Step 3: Build and verify CLI works**

Run: `just build` Run:
`./build/langlang codegen -grammar grammars/sku_record.peg -lang go -output-path /dev/stdout -go-package wire`
Expected: Prints generated Go code.

**Step 4: Commit**

    feat: add 'langlang codegen' subcommand for binary struct codegen

--------------------------------------------------------------------------------

### Task 10: End-to-end verification

**Step 1: Run full test suite**

Run: `just test` Expected: All tests pass including new binary tests and all
existing tests.

**Step 2: Verify the generate/test cycle**

Run: `just generate && just test` Expected: Bootstrap regeneration succeeds, all
tests pass.

**Step 3: Final commit if any loose ends**

Only if there are formatting or minor fixes needed.
