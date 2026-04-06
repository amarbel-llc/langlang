# WAT Codegen Target Design

**Date:** 2026-04-06 **Issue:** [#18 --- FDR-0006 MVP: binary codegen for WASM
host-guest wire format](https://github.com/amarbel-llc/langlang/issues/18)
(follow-up comment) **Status:** proposed

## Problem

The Go codegen for the dodder wire format is done. The guest side (WAT) still
requires hand-written binary parsing code. Langlang should generate the parsing
glue so both host and guest update from one grammar.

## Non-goals

- WIT / Component Model compatibility. wazero does not support Component Model,
  and the canonical ABI (pointer-based random access) is incompatible with the
  length-prefixed sequential format this feature uses.
- Rust guest codegen. Focused on Go host ↔ WAT guest only.
- Generating filter/business logic. Only the decode glue. The guest developer
  still writes their filter logic in hand-written WAT.

## Design Decisions

- **Result struct in linear memory**, not mutable globals. The decode function
  takes a caller-provided result pointer and writes field ptr/len pairs there.
  Stateless and re-entrant; the guest owns the memory region.
- **WAT fragment, not full module.** Output has no `(module ...)` wrapper so it
  can be `(@use)`/`(@include)` or concatenated into a filter module.
- **Top-level bindings get result slots.** List items are accessed via a
  generated `$list_item_at` helper, not individual slots.

## Result Struct Layout

One 8-byte slot per top-level binding:

- `prefixed_string` field: `(ptr: i32, len: i32)` --- 8 bytes
- `prefixed_string_list` field: `(ptr: i32, count: i32)` --- 8 bytes where `ptr`
  points to the start of the list (past its count prefix)

For `sku_record` (7 fields): 56 bytes total.

Layout constants are generated as comments at the top of the WAT output for the
hand-written caller to reference.

    ;; SkuRecord result layout (56 bytes):
    ;;   offset  0: genre_ptr         (i32)
    ;;   offset  4: genre_len         (i32)
    ;;   offset  8: object_id_ptr     (i32)
    ;;   offset 12: object_id_len     (i32)
    ;;   offset 16: type_id_ptr       (i32)
    ;;   offset 20: type_id_len       (i32)
    ;;   offset 24: tags_ptr          (i32)  ;; list start (past count)
    ;;   offset 28: tags_count        (i32)
    ;;   offset 32: tags_implicit_ptr (i32)
    ;;   offset 36: tags_implicit_count (i32)
    ;;   offset 40: blob_digest_ptr   (i32)
    ;;   offset 44: blob_digest_len   (i32)
    ;;   offset 48: description_ptr   (i32)
    ;;   offset 52: description_len   (i32)

## Decode Function

Generated for each rule that has name bindings:

``` wat
(func $decode_sku_record
  (param $buf i32) (param $buf_len i32) (param $result i32)
  (local $off i32)
  (local $str_len i32)
  (local $list_count i32)

  ;; genre: prefixed_string
  ;; read u32le length
  (local.set $str_len
    (i32.load (i32.add (local.get $buf) (local.get $off))))
  (local.set $off (i32.add (local.get $off) (i32.const 4)))
  ;; write genre_ptr (points to data, past the length prefix)
  (i32.store offset=0 (local.get $result)
    (i32.add (local.get $buf) (local.get $off)))
  ;; write genre_len
  (i32.store offset=4 (local.get $result) (local.get $str_len))
  ;; advance past data
  (local.set $off (i32.add (local.get $off) (local.get $str_len)))

  ;; object_id: prefixed_string
  ;; ... same pattern with result offsets 8 and 12

  ;; tags: prefixed_string_list
  ;; read u32le count
  (local.set $list_count
    (i32.load (i32.add (local.get $buf) (local.get $off))))
  (local.set $off (i32.add (local.get $off) (i32.const 4)))
  ;; write tags_ptr (points past the count, at the first item)
  (i32.store offset=24 (local.get $result)
    (i32.add (local.get $buf) (local.get $off)))
  ;; write tags_count
  (i32.store offset=28 (local.get $result) (local.get $list_count))
  ;; skip over list items by walking each prefixed_string
  (call $skip_prefixed_string_list
    (i32.add (local.get $buf) (local.get $off))
    (local.get $list_count))
  ;; advance off by the returned total size
  ;; ... details in implementation

  ;; ... rest of fields
)
```

The decode function signature is:
`(param $buf i32) (param $buf_len i32) (param $result i32)`. Returns nothing
(the result struct is populated by store instructions).

`$buf_len` is used for bounds checking (truncated MVP may skip this).

## Helper Functions

### `$list_item_at`

Access item N from a `prefixed_string_list`:

``` wat
(func $list_item_at
  (param $list_ptr i32) (param $index i32)
  (result i32 i32)  ;; returns (data_ptr, data_len)
  (local $off i32)
  (local $i i32)
  (local $len i32)
  ;; walk forward, skipping $index items
  (loop $walk
    (br_if $walk (i32.ge_s (local.get $i) (local.get $index))
      ;; read len at offset, advance past len+data
      (local.set $len (i32.load (i32.add (local.get $list_ptr) (local.get $off))))
      (local.set $off (i32.add (local.get $off) (i32.add (i32.const 4) (local.get $len))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
    )
  )
  ;; at target item: read len, return (ptr past len, len)
  (local.set $len (i32.load (i32.add (local.get $list_ptr) (local.get $off))))
  (i32.add (local.get $list_ptr) (i32.add (local.get $off) (i32.const 4)))
  (local.get $len)
)
```

### Sub-rule decode functions

For each rule with bindings, a decode function is generated. `prefixed_string`
and `prefixed_string_list` also get decode functions (as internal helpers) that
the `sku_record` decoder calls --- OR the generation inlines them. For MVP,
**inline**: simpler codegen, no function call overhead, only one public
`$decode_sku_record` function.

## CLI

Extend existing `langlang codegen` subcommand:

    langlang codegen -grammar sku_record.peg -lang wat -output-path sku_record.wat

The `-go-package` flag is ignored when `-lang wat`. No new flags needed.

## Test Strategy

Go integration test using wazero (new dependency):

1.  Generate WAT fragment from `sku_record.peg`
2.  Wrap in a module with memory + exported decode function
3.  Compile WAT → WASM via wazero
4.  Instantiate the module
5.  For each test case:
    a.  Encode a `SkuRecord` via existing Go `EncodeSkuRecord`
    b.  Write encoded bytes into WASM linear memory at offset 0
    c.  Write zero bytes at result offset (say, 8192)
    d.  Call `$decode_sku_record(0, len, 8192)`
    e.  Read result struct from WASM memory
    f.  For each field, read the ptr/len from result, read the data from buf,
        verify it matches the original `SkuRecord` field

Test file: `go/tests/binary/wat_test.go`. Reuses generated `wire.go` from the
existing binary integration tests for the Go encode side.

## Module Wrapper (Test Only)

The generated fragment needs wrapping for the test:

``` wat
(module
  (memory (export "memory") 1)
  ;; === Generated fragment ===
  ;; ... globals (none), decode function, helpers ...
  (export "decode_sku_record" (func $decode_sku_record))
)
```

The test builds this wrapper by concatenating strings around the generated
fragment. The codegen itself does NOT emit the wrapper --- it's purely a
fragment.

## Implementation Sketch

New files: - `go/binary/gen_wat.go` --- WAT codegen, mirrors the structure of
`gen.go` - `go/binary/gen_wat_test.go` --- unit tests asserting WAT text
patterns - `go/tests/binary/wat_test.go` --- wazero integration test

Modified files: - `go/cmd/langlang/codegen.go` --- add `wat` to `-lang`
options - `go/go.mod` --- add `github.com/tetratelabs/wazero` dependency

Key codegen types (reuse from `gen.go`): - `ruleInfo`, `fieldInfo`, `seqItem`
--- same analysis applies to WAT output

New functions in `gen_wat.go`: -
`GenerateWAT(grammar *langlang.GrammarNode, opts Options) (string, error)` -
`emitWATResultLayoutComment(buf, info)` -
`emitWATDecodeFunc(buf, info, allRules)` --- inlines sub-rule decoding -
`emitWATListHelper(buf)` --- emits `$list_item_at`

## Rollback

Purely additive: - New `-lang wat` value in existing subcommand - New files in
`go/binary/` and `go/tests/binary/` - New wazero dependency (test-only)

Revert commits to remove. No existing behavior changes.

## Future Work

- `$list_item_at` for `prefixed_string_list` of other types (currently only
  `prefixed_string`)
- Nested records within records (beyond what `sku_record` needs)
- Bounds checking using `$buf_len` parameter
- Error return values (currently decode assumes well-formed input)
