---
date: 2026-03-27
promotion-criteria: PNG grammar parses valid PNG files and round-trips (decode
  then encode) to identical bytes. MessagePack grammar parses basic types
  correctly. Rust codegen emits compilable decoder with borrowed output for at
  least one binary grammar.
status: proposed
---

# Binary Parsing and Serialization

## Problem Statement

Langlang's VM operates on `[]byte` with a byte-offset cursor -- nothing in the
instruction set assumes text input. However, the grammar syntax lacks primitives
for binary data: fixed-width integers, hex byte literals, and length-prefixed
fields. Adding these, combined with the typed extraction codegen (FDR-0001),
turns langlang into a grammar-driven binary parser generator that can outperform
reflection-based frameworks (protobuf, encoding/binary) for formats with complex
conditional structure.

## Interface

**Grammar syntax additions:**

``` peg
@binary                           # suppress implicit whitespace
@endian big                       # default endianness for shorthand

PNGSignature <- x"89504E470D0A1A0A"   # hex byte literals
Width        <- u32be                  # fixed-width numeric primitives
ChunkLength  <- u32be -> length        # value binding
ChunkData    <- bytes(length)          # length-prefixed field
```

**Numeric primitives:** `u8`, `u16be/le`, `u32be/le`, `u64be/le`, `i8`,
`i16be/le`, `i32be/le`, `i64be/le`, `f32be/le`, `f64be/le`, `varint`.

**Value binding:** `-> name` binds a parsed numeric value; `bytes(name)`
consumes that many bytes. This is the one departure from context-free PEG.

**Bidirectional codegen:** Binary grammars specify exact byte layout, enabling
both decoder (bytes to struct) and encoder (struct to bytes) generation from the
same grammar.

**Rust codegen backend:** A new `--target rust` flag emits Rust modules with
lifetime-parameterized structs for zero-copy borrowed output, `#[repr(C)]`
transmute for wire-compatible fixed layouts, and no-std support.

``` sh
langlang generate --target rust --grammar png.peg --output src/png_parser.rs
```

## Examples

PNG chunk grammar:

``` peg
@binary
@endian big

PNG       <- Signature Chunk+
Signature <- x"89504E470D0A1A0A"
Chunk     <- u32 -> length ChunkType bytes(length) u32
ChunkType <- u8 u8 u8 u8
```

Generated Go decoder extracts directly from bytes:

``` go
out.Width = binary.BigEndian.Uint32(input[n.start:n.end])
```

Generated Rust decoder with borrowed output:

``` rust
pub struct PNGChunk<'a> {
    pub length: u32,
    pub chunk_type: [u8; 4],
    pub data: &'a [u8],    // borrows directly from input
    pub crc: u32,
}
```

## Limitations

- Value binding (`-> name` / `bytes(name)`) introduces context-sensitivity.
  Scope is the enclosing rule and its children.
- No computed expressions on bound values (`bytes(length - 4)`) in initial
  version. Arithmetic handled in extraction codegen.
- No bit-level parsing. Sub-byte fields (TCP flags, compression headers) require
  a separate extension.
- Cannot parse formats with embedded compression (gzip payloads) or encryption
  (TLS application data) -- grammar handles the container only.
- Self-describing formats (protobuf with unknown fields) are not a fit;
  grammar-derived parsers expect all bytes to conform.
- Rust codegen invokes the Go langlang CLI from `build.rs`, creating a
  cross-language build dependency.

## MVP Notes (2026-04-05)

- **`@binary` dropped:** The input is always a bytestream; binary primitives are
  new expression types valid in any grammar. Use `@whitespace none` explicitly.
- **`@endian` deferred:** Each numeric primitive encodes endianness in its name
  (`u32le`, `u32be`). A default-endianness directive may be added later if
  grammars get verbose with repeated `le`/`be` suffixes.
- **Binding syntax:** `name:expr` instead of `expr -> name`. Reads
  left-to-right: the name labels what follows.
- **MVP scope:** Go codegen only. Rust codegen, cross-language tests, signed
  integers, floats, hex literals, and varint deferred.

See [binary-codegen-design.md](../plans/2026-04-05-binary-codegen-design.md) for
the full MVP design.

## More Information

- Full design with VM extensions, output model options, encoder generation,
  format expressibility analysis, and Rust codegen architecture:
  [langlang-binary-serialization.md](../references/langlang-binary-serialization.md)
- Extends: [FDR-0001 Typed Tree Extraction](0001-typed-tree-extraction.md)
  (binary fields are additions to the FieldKind taxonomy)
- Related: [FDR-0002 Typed Arena Strategies](0002-typed-arena-strategies.md)
  (binary fields are small/fixed-size, favoring Strategy B)
- Related: [FDR-0003 Divide-and-Conquer
  Parsing](0003-divide-and-conquer-parsing.md) (binary TLV containers have
  scannable type bytes)
- Related: [FDR-0005 SIMD Junction Scanner](0005-simd-junction-scanner.md)
  (binary format scanners classify type bytes with same VPSHUFB technique)
- Comparison targets: nom (Rust), kaitai struct, deku (Rust),
  protobuf/flatbuffers/cap'n proto
