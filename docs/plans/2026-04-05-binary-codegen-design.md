# Binary Codegen MVP Design

**Date:** 2026-04-05 **Issue:** [#18 --- FDR-0006 MVP: binary codegen for WASM
host-guest wire format](https://github.com/amarbel-llc/langlang/issues/18)
**Status:** proposed

## Problem

dodder needs to pass structured data between a Go host (wazero) and WASM guests.
The current approach is \~150 lines of hand-rolled canonical ABI pointer
arithmetic. The WIT Component Model would generate this code, but wazero does
not support it.

langlang can define the wire format as a PEG grammar and generate Go
encoder/decoder from a single source of truth.

## Scope

- Grammar parser extensions for binary primitives
- Go struct + Decode/Encode/Size codegen from grammar AST (direct, no VM)
- Go round-trip tests using the dodder `sku_record` grammar
- No Rust codegen, no cross-language tests (deferred)

## Design Decisions

- **No `@binary` directive.** The input is always a bytestream; binary
  primitives are just new expression types valid in any grammar. The dodder
  grammar uses `@whitespace none` explicitly.
- **No `@endian` directive.** Each numeric primitive encodes its endianness in
  its name (`u32le`, `u32be`). A default-endianness directive may be added later
  if grammars get verbose.
- **Standalone structs.** Codegen emits struct definitions derived from grammar
  bindings. No user-defined structs with `ll:` tags (the extract pattern). The
  `ll:` tag approach is worth exploring post-MVP for mapping binary formats onto
  existing Go types.
- **New subcommand.** `langlang codegen` rather than extending `generate`. The
  existing `generate` embeds a VM; `codegen` emits direct functions.

## Grammar Parser Extensions

Four new AST node types in `grammar_ast.go`, available in all grammars:

### NumericPrimitiveNode

Consumes a fixed number of bytes and interprets them as a numeric value.

    u8, u16le, u16be, u32le, u32be, u64le, u64be

Fields: `Name string` (e.g. "u32le"), `Width int` (bytes), `BigEndian bool`.

### NameBindingNode

Binds a name to the result of an expression. Syntax: `name:expr`.

``` peg
len:u32le
```

Fields: `Name string`, `Expr AstNode`.

Distinct from `LabeledNode` (`expr^label`) which is for error labels.

### BytesConsumeNode

Consumes N bytes where N is the value of a previously-bound name. Syntax:
`bytes(name)`.

``` peg
data:bytes(len)
```

Fields: `Name string` (references a binding).

### CountedRepetitionNode

Repeats an expression N times where N is the value of a previously-bound name.
Syntax: `expr{count}`.

``` peg
items:prefixed_string{count}
```

Fields: `Expr AstNode`, `Count string` (references a binding).

## Go Codegen

New package: `go/binary/`

### Input

`*GrammarNode` from the query system (`QueryAST`).

### Output

A single `.go` file containing:

- **One struct per rule** with exported fields derived from `NameBindingNode`
  bindings. Field types are determined by the bound expression:

  - `NumericPrimitiveNode` → `uint8`, `uint16`, `uint32`, `uint64`
  - `BytesConsumeNode` → `[]byte`
  - `CountedRepetitionNode` → `[]T` where T is the repeated rule's struct type
  - `IdentifierNode` (rule reference) → the referenced rule's struct type

- **`DecodeRuleName(data []byte) (RuleName, int, error)`** --- decodes from
  bytes, returns struct + bytes consumed + error.

- **`EncodeRuleName(v *RuleName) ([]byte, error)`** --- serializes struct to
  bytes.

- **`SizeRuleName(v *RuleName) int`** --- returns encoded byte size without
  allocating.

### Codegen Strategy

Walk `SequenceNode` items in order, tracking byte offset:

- `NumericPrimitiveNode` → `binary.LittleEndian.Uint32(data[off:off+4])`
- `BytesConsumeNode` → `copy(out, data[off:off+n])`
- `CountedRepetitionNode` → loop calling the referenced rule's decoder
- `IdentifierNode` → call the referenced rule's decoder

Encoder mirrors the decoder: write length, write data, advance offset.

## CLI

New subcommand: `langlang codegen`

    langlang codegen -grammar=foo.peg -lang=go -output-path=./wire.go -go-package=wire

Flags: - `-grammar` --- path to grammar file (required) - `-lang` --- output
language, only `go` for MVP (required) - `-output-path` --- output file path
(required) - `-go-package` --- Go package name (default: `wire`)

## Test Grammar

`grammars/sku_record.peg`:

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

## Tests

1.  **AST parsing test:** Parse `sku_record.peg`, assert correct node types and
    structure (bindings, primitives, bytes, counted repetition).

2.  **Round-trip test:** Create a `SkuRecord` with known values → encode →
    decode → assert equal. Edge cases: empty strings, empty tag lists.

3.  **Golden bytes test:** Encode a known struct, compare against hand-computed
    byte sequence to catch endianness/length-prefix bugs.

All tests run via `just test`.

## Rollback

Purely additive: new AST nodes, new package, new subcommand. Rollback = revert
commits. No existing behavior changes.

## Future Work

- `@endian` directive for default endianness (avoids repeating `le`/`be` on
  every primitive)
- Signed integers (`i8`, `i16le`, etc.) and floats (`f32le`, etc.)
- Rust codegen with zero-copy `&'a [u8]` borrowed output
- `ll:` tag integration for mapping binary formats onto user-defined Go structs
- Cross-language round-trip tests (Go encode ↔ Rust decode)
