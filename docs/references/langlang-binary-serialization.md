# Langlang as a Binary Parsing and Serialization Framework

## Concept

Langlang's VM already operates on `[]byte` with a byte-offset cursor. Nothing
in the instruction set assumes text input. The grammar syntax and output model
are text-oriented, but the runtime is byte-generic. Extending the grammar
syntax with binary primitives (fixed-width integers, byte literals in hex,
length-prefixed fields) and adding a small value stack to the VM would turn
langlang into a grammar-driven binary parser generator.

Combined with the typed extraction codegen, this produces a full serialization
framework: the grammar is the schema, the codegen produces both decoder and
encoder, and the generated code can outperform reflection-based serialization
libraries (protobuf, encoding/binary) for formats with complex conditional
structure.

## What's Already Binary-Ready

The VM core:

- Cursor is a byte offset (`int`), not a character offset
- `IChar` matches a single byte value
- `ICharset` tests a byte against a 256-bit bitset (all 256 byte values)
- The arena stores `(start, end)` byte offsets into the input
- `Tree.Text(id)` returns `string(input[start:end])` — Go strings are byte
  sequences, not character sequences

The Tree output:

- Nodes record byte spans, not character spans
- The `Span` type has `Location.Cursor` (byte offset) as its primary
  coordinate; line/column are derived

The codegen infrastructure:

- `GenGoEval` embeds the VM into the consumer's package — the consumer
  controls the full binary
- `QueryAST` provides programmatic access to the grammar structure at
  generation time
- The typed extraction plan (existing) generates struct-populating code from
  grammar + Go types

## What Needs to Change

### Grammar Syntax

Three additions to the PEG grammar syntax:

**1. Byte literals in hex**

```peg
@binary                             # directive: disable whitespace handling

PNGSignature <- x"89504E470D0A1A0A"  # match 8 specific bytes
NullByte     <- x"00"               # match single byte by hex value
```

The `x"..."` syntax produces a sequence of `IChar` instructions, one per byte.
No whitespace interleaving (the `@binary` directive suppresses implicit
whitespace insertion between sequence elements globally).

**2. Fixed-width numeric primitives**

```peg
Width    <- u32be     # 4 bytes, unsigned, big-endian
Height   <- u32be
BitDepth <- u8        # 1 byte, unsigned
Offset   <- i32le     # 4 bytes, signed, little-endian
Ratio    <- f64le     # 8 bytes, IEEE 754 double, little-endian
```

These are new terminal rules built into the grammar. Each advances the cursor
by a fixed number of bytes and produces a typed value. The full set:

| Primitive | Bytes | Description                          |
|-----------|-------|--------------------------------------|
| `u8`      | 1     | unsigned 8-bit                       |
| `u16be`   | 2     | unsigned 16-bit, big-endian          |
| `u16le`   | 2     | unsigned 16-bit, little-endian       |
| `u32be`   | 4     | unsigned 32-bit, big-endian          |
| `u32le`   | 4     | unsigned 32-bit, little-endian       |
| `u64be`   | 8     | unsigned 64-bit, big-endian          |
| `u64le`   | 8     | unsigned 64-bit, little-endian       |
| `i8`      | 1     | signed 8-bit                         |
| `i16be`   | 2     | signed 16-bit, big-endian            |
| `i16le`   | 2     | signed 16-bit, little-endian         |
| `i32be`   | 4     | signed 32-bit, big-endian            |
| `i32le`   | 4     | signed 32-bit, little-endian         |
| `i64be`   | 8     | signed 64-bit, big-endian            |
| `i64le`   | 8     | signed 64-bit, little-endian         |
| `f32be`   | 4     | IEEE 754 float, big-endian           |
| `f32le`   | 4     | IEEE 754 float, little-endian        |
| `f64be`   | 8     | IEEE 754 double, big-endian          |
| `f64le`   | 8     | IEEE 754 double, little-endian       |
| `varint`  | 1-10  | Protocol Buffers-style variable int  |

The `@endian` directive sets a default:

```peg
@binary
@endian big       # u32 is shorthand for u32be in this grammar

Width  <- u32     # equivalent to u32be
Height <- u32
```

**3. Length-prefixed fields (value binding)**

```peg
ChunkLength <- u32be -> length     # parse u32, bind result to 'length'
ChunkData   <- bytes(length)       # consume 'length' bytes
```

The `-> name` syntax binds a parsed numeric value to a name. The
`bytes(name)` syntax consumes that many bytes. This is the one departure from
context-free PEG: the number of bytes consumed depends on a previously parsed
value.

The compiler resolves `length` to a value stack index at compile time, the
same way it resolves rule names to string table IDs.

### VM Extensions

**Value stack**

A new stack alongside the existing capture stack and backtracking stack.
`IFixedWidth` pushes a value. `IReadLength` / `bytes(name)` pops a value.
The value stack participates in backtracking — when the VM rewinds, the value
stack rewinds with it.

```go
type virtualMachine struct {
    // ... existing fields ...
    values    []uint64
    valuesTop int
}
```

Three new instructions:

```go
const (
    // IFixedWidth: consume N bytes, interpret as numeric, push to value stack.
    // Operands: width (1/2/4/8), encoding (unsigned/signed/float), endianness
    IFixedWidth InstructionType = ...

    // IBindValue: copy top of value stack to a named slot (for later reference)
    IBindValue InstructionType = ...

    // IConsumeBytes: pop a value from a named slot, consume that many bytes
    IConsumeBytes InstructionType = ...
)
```

**@binary directive handling**

When `@binary` is active, the compiler suppresses all whitespace insertion
between sequence elements. The existing `@whitespace` directive already
controls this; `@binary` is sugar for `@whitespace none` plus enabling the
binary primitive keywords.

### Output Model

The current tree stores byte spans. For text parsing, the consumer calls
`Text(id)` to get a string. For binary parsing, the consumer needs typed
numeric values.

Two options:

**Option A: Value in the tree.** Add a `NodeType_Value` that stores a
`uint64` directly in the node (alongside start/end). The tree becomes:

```go
type node struct {
    typ       NodeType
    start     int
    end       int
    nameID    int32
    childID   int32
    messageID int32
    value     uint64  // populated for NodeType_Value nodes
}
```

This adds 8 bytes to every node (36 bytes total), even nodes that don't carry
a numeric value. For text grammars, wasted space. For binary grammars where
most nodes are numeric fields, it's the right trade-off.

**Option B: Typed extraction only.** Don't store values in the tree. The
tree records byte spans as usual. The generated extraction code reads the
bytes directly:

```go
// Generated:
out.Width = binary.BigEndian.Uint32(input[n.start:n.end])
```

This is zero overhead on the tree and works with the existing arena layout.
The extraction codegen needs to know the field type (from the grammar's
`u32be` primitive) to emit the correct `binary.BigEndian.*` call.

Option B is preferred. It keeps the tree model unchanged and puts the type
interpretation in the generated extraction code, where it belongs.

## Bidirectional Codegen: Decoder and Encoder

Text grammars produce decoders only. Encoding text requires formatting
decisions (whitespace, comments, indentation) that the grammar can't fully
specify.

Binary grammars specify exact byte layout. The grammar contains enough
information to generate both a decoder (bytes → struct) and an encoder
(struct → bytes).

### Encoder generation

For each rule that maps to a struct field, the codegen emits the inverse
operation:

| Grammar construct       | Decoder (generated)                                           | Encoder (generated)                                            |
|-------------------------|---------------------------------------------------------------|----------------------------------------------------------------|
| `u32be`                 | `binary.BigEndian.Uint32(input[s:e])`                         | `binary.BigEndian.PutUint32(buf[off:], val)`                   |
| `x"89504E47"`           | `bytes.Equal(input[s:e], []byte{0x89,0x50,0x4E,0x47})`       | `copy(buf[off:], []byte{0x89,0x50,0x4E,0x47})`                |
| `u32be -> length`       | read u32, bind to length                                      | write u32 from struct field                                    |
| `bytes(length)`         | `input[s:s+length]`                                           | `copy(buf[off:], data); off += len(data)`                      |
| Choice `A / B`          | match first succeeding alternative                            | encode whichever variant is populated                          |
| Repetition `Item*`      | collect slice                                                 | encode each element sequentially                               |

The generated encoder:

```go
func EncodePNGChunk(chunk PNGChunk) ([]byte, error) {
    size := 4 + 4 + len(chunk.Data) + 4  // computed from grammar structure
    buf := make([]byte, size)
    off := 0

    binary.BigEndian.PutUint32(buf[off:], chunk.Length)
    off += 4

    copy(buf[off:], chunk.Type[:])
    off += 4

    copy(buf[off:], chunk.Data)
    off += len(chunk.Data)

    binary.BigEndian.PutUint32(buf[off:], chunk.CRC)
    off += 4

    return buf, nil
}
```

For fixed-size formats, the `size` computation is a compile-time constant.
For variable-size formats (length-prefixed fields, repetitions), the encoder
does a pre-pass to compute total size, allocates once, then fills.

### Encoder size computation

The codegen generates a `Size` function alongside `Encode`:

```go
func SizePNGChunk(chunk PNGChunk) int {
    return 4 + 4 + len(chunk.Data) + 4
}
```

For deeply nested formats, the size function recurses over the struct tree.
This is the same pattern as protobuf's `Size()` method.

## Performance: Where This Beats Existing Frameworks

### Conditional structure

Protobuf, flatbuffers, and cap'n proto encode data in a uniform tag-value
(TLV — Tag-Length-Value) layout. Every field has the same structural overhead
regardless of context. Parsing always decodes tag, dispatches on field number,
decodes value.

Many real binary formats have conditional structure: "if the type byte is 0x01,
the next 8 bytes are a timestamp; if it's 0x02, the next 4 bytes are a count
followed by count×12-byte records." PEG's ordered choice expresses this
directly:

```peg
Record <- TypeTimestamp / TypeRecordBatch
TypeTimestamp   <- x"01" i64be
TypeRecordBatch <- x"02" u32be -> count Record[count]
```

The generated decoder for this has a single byte-comparison branch. No tag
decoding, no field number dispatch, no varint parsing for the tag. For
formats with many conditional branches (MessagePack, CBOR, TLS), the
grammar-derived decoder eliminates the generic dispatch overhead that
reflection-based libraries pay.

### Zero-copy for byte slices

Length-prefixed byte fields (`bytes(length)`) can be extracted as
`input[start:start+length]` — a slice of the original input, no copy. The
generated extraction code knows at compile time which fields are byte slices
and emits slice expressions instead of `copy()` calls. This is the same
technique cap'n proto and flatbuffers use (in-place access), but derived
from the grammar rather than hardcoded in a runtime library.

### Branchless field access

For fixed-layout records (no optional fields, no choices), the generated
decoder is a straight-line sequence of `binary.BigEndian.UintXX` calls at
computed offsets. No branches, no loops, no hash table lookups. The Go
compiler can further optimize this into SIMD loads on architectures that
support unaligned access.

### No reflection

Protobuf in Go uses reflection for the generic `proto.Unmarshal` path.
The generated `.pb.go` code avoids some reflection but still uses interfaces
and type switches. Langlang's generated decoder has no reflection, no
interfaces, and no type switches — it's straight-line code that reads bytes
and writes struct fields.

## Performance: Where This Loses

### Self-describing formats

Protobuf, Avro (with schema registry), and similar formats can skip
unknown fields without understanding them — the wire format is self-describing
enough to determine field boundaries. A grammar-derived parser expects every
byte to conform to the grammar. Unknown fields cause a parse error.

This is a fundamental trade-off: grammar-derived parsers are fast for known
formats and fail on unknown extensions. TLV formats are slower for known
formats but gracefully handle unknown fields.

For closed formats (PNG, PCAP, git packfiles, game save files, sensor data),
the grammar-derived approach wins. For open formats (protobuf over a
network where the schema may have new fields), the TLV approach is safer.

### Varint encoding

Protobuf's varint encoding is a byte-at-a-time loop with a 7-bit-per-byte
packing scheme. The grammar-derived parser would express this as:

```peg
Varint <- HighByte* LowByte
HighByte <- [x80-xFF]   # MSB set: more bytes follow
LowByte  <- [x00-x7F]   # MSB clear: last byte
```

This is correct but produces a sequence of individual byte captures. The
generated extraction code must reconstitute the integer from the captured
bytes. A hand-written varint decoder does this in a tight loop with
shift-and-OR. The grammar-derived version adds tree-building overhead per
byte.

Mitigation: add `varint` as a built-in primitive (like `u32be`), implemented
as a specialized VM instruction that parses and produces a numeric value in
one step. The grammar syntax treats it as an opaque terminal; the VM handles
the byte-at-a-time loop internally.

## Existing Binary Formats

### Directly expressible (pure PEG + binary extensions)

**PNG**: sequence of chunks, each with length-type-data-CRC. Clean TLV.
The grammar is ~20 lines.

```peg
@binary
@endian big

PNG       <- Signature Chunk+
Signature <- x"89504E470D0A1A0A"
Chunk     <- u32 -> length ChunkType bytes(length) u32
ChunkType <- u8 u8 u8 u8
```

**MessagePack**: type byte followed by type-dependent payload. The first
byte determines the structure. This is an ordered choice on byte ranges —
pure PEG.

```peg
@binary

Value    <- Nil / False / True / PosFixInt / NegFixInt
           / UInt8 / UInt16 / UInt32 / UInt64
           / FixStr / Str8 / Str16 / Str32
           / FixArray / Array16 / Array32
           / FixMap / Map16 / Map32
           / ...

Nil        <- x"C0"
False      <- x"C2"
True       <- x"C3"
PosFixInt  <- [x00-x7F]
NegFixInt  <- [xE0-xFF]
UInt8      <- x"CC" u8
UInt16     <- x"CD" u16be
FixStr     <- [xA0-xBF] -> length bytes(length & 0x1F)
```

**CBOR (Concise Binary Object Representation)**: similar structure to
MessagePack. Type and additional info encoded in the first byte.

**TLS record layer**: type (1 byte), version (2 bytes), length (2 bytes),
fragment (length bytes). Textbook TLV.

**ELF headers**: fixed-layout structs with endianness determined by a
magic byte. The grammar can use a choice on the endianness byte:

```peg
@binary

ELF       <- ELFMagic ELFClass ELFData ELFVersion ...
ELFMagic  <- x"7F454C46"
ELFClass  <- x"01" / x"02"        # 32-bit / 64-bit
ELFData   <- x"01" / x"02"        # little / big endian
```

**Git packfile**: header (signature + version + object count), followed by
object entries, followed by a trailing checksum. Object entries have a
type-and-size variable-length header. Expressible with value binding.

### Partially expressible (need extensions)

**Protobuf wire format**: the varint tag encoding needs a built-in `varint`
primitive. The field-number dispatch is an ordered choice. The actual field
semantics require a `.proto` schema — the wire format alone doesn't tell you
field names or types. Langlang can parse the wire format but needs the
`.proto` to assign meaning.

**DNS packets**: mostly expressible. Name compression (pointers that reference
earlier positions in the packet) requires random access — the cursor would
need to jump backward. PEG's cursor only moves forward. The compressed names
would need to be parsed as raw bytes and decompressed in a post-processing
step.

**ZIP archives**: the central directory at the end of the file references
local file headers at arbitrary offsets. This requires bidirectional
parsing or multi-pass. The individual structures (local file header, central
directory entry) are straightforward TLV.

### Not expressible

**Formats with embedded compression**: gzip, zstd-compressed blocks within
a container. The grammar can describe the container but not decompress the
payload.

**Formats with encryption**: TLS application data after the handshake. The
grammar can parse the record layer but not the encrypted content.

**Self-modifying formats**: formats where a field's interpretation depends
on runtime state beyond what's in the input (e.g., a format that requires
a lookup table loaded from an external file).

## Rust Codegen

All explorations so far have targeted Go codegen. Langlang has a Rust
codebase (`rust/` in the repo) with a VM implementation (`langlang_lib`),
a grammar parser (`langlang_syntax`), and a value/tree type system
(`langlang_value`). However, the Rust side currently has no ahead-of-time
codegen equivalent to Go's `GenGoEval`. It's VM-only — you compile a grammar
at runtime and execute it with the interpreter.

Rust codegen is a natural fit for binary parsing and arguably a better target
than Go for several reasons.

### Why Rust codegen matters for binary parsing

**Zero-cost abstractions.** Rust's ownership model means the generated
decoder can return structs that borrow directly from the input buffer
(`&'a [u8]` slices) without lifetime ambiguity. In Go, a `[]byte` slice
into the input has no compile-time guarantee that the input outlives the
slice. In Rust, the borrow checker enforces this.

```rust
// Generated Rust decoder — borrows from input, zero-copy
pub struct PNGChunk<'a> {
    pub length: u32,
    pub chunk_type: [u8; 4],
    pub data: &'a [u8],    // borrows directly from input
    pub crc: u32,
}

pub fn decode_png_chunk(input: &[u8]) -> Result<(PNGChunk, &[u8]), Error> {
    let length = u32::from_be_bytes(input[0..4].try_into()?);
    let chunk_type: [u8; 4] = input[4..8].try_into()?;
    let data = &input[8..8 + length as usize];
    let crc_start = 8 + length as usize;
    let crc = u32::from_be_bytes(input[crc_start..crc_start + 4].try_into()?);
    let rest = &input[crc_start + 4..];
    Ok((PNGChunk { length, chunk_type, data, crc }, rest))
}
```

The `'a` lifetime ties `data` to the input buffer. The compiler rejects
any code that uses `PNGChunk` after the input is freed. Go can't express this.

**No GC pauses.** Binary parsing of large inputs (multi-MB PCAP captures,
firmware images, database pages) can produce millions of nodes. In Go, these
are GC-tracked allocations. In Rust, they're stack-allocated or arena-allocated
with deterministic deallocation. For sustained high-throughput parsing (network
packet processing, database page decoding), the absence of GC pauses is
significant.

**`#[repr(C)]` for wire-compatible structs.** For binary formats whose
in-memory layout matches the wire format (flat structs, no padding, known
endianness), Rust can mark the struct `#[repr(C)]` and transmute the input
bytes directly into a typed reference:

```rust
#[repr(C, packed)]
struct RawHeader {
    magic: [u8; 4],
    version: u16,
    flags: u16,
    length: u32,
}

fn decode_header(input: &[u8]) -> &RawHeader {
    // Safety: RawHeader is repr(C, packed), input is aligned and long enough
    unsafe { &*(input.as_ptr() as *const RawHeader) }
}
```

This is zero-copy AND zero-work — no field-by-field decoding at all. The
codegen can emit this for fixed-layout portions of a grammar where the struct
layout matches the wire format. The grammar analysis determines whether a
rule's byte layout matches `#[repr(C)]` constraints at generation time.

**SIMD without FFI.** Rust has first-class SIMD intrinsics via
`std::arch::x86_64` (and ARM NEON via `std::arch::aarch64`). The junction
scanner can be written in safe Rust with architecture-specific intrinsics,
without needing an external assembly generator like avo. The `#[target_feature]`
attribute handles runtime CPU detection:

```rust
#[target_feature(enable = "avx2")]
unsafe fn scan_junctions_avx2(input: &[u8]) -> Vec<JunctionHit> {
    // VPSHUFB-based classification, same algorithm as the Go/avo version
    // but using std::arch::x86_64::_mm256_shuffle_epi8 etc.
}
```

This is portable within Rust's type system — the same crate works on x86-64
(AVX2), ARM64 (NEON), and WASM (SIMD128) with `cfg` attributes selecting
the implementation.

### Rust codegen architecture

The Rust codegen would be a new backend for langlang's grammar compiler. The
compiler already produces a `Program` (IR — Intermediate Representation) and
`Bytecode`. A Rust backend would target either the `Program` level (emit Rust
source that mirrors the IR's instruction structure) or bypass the IR entirely
and emit Rust directly from the grammar AST.

```
grammar.peg
    ↓  langlang grammar parser → GrammarNode (AST)
    ↓
    ├→ Go backend (existing gen.go)
    │  → embeds VM + bytecode into Go package
    │
    ├→ Go extraction backend (from extraction plan)
    │  → emits typed struct extraction code
    │
    └→ Rust backend (new)
       → emits Rust module with:
         - parser function (no VM — direct match logic)
         - typed structs with lifetime parameters
         - decode function (input → borrowed struct)
         - encode function (struct → Vec<u8>)
         - optional SIMD scanner
```

The Rust backend would be invoked from the langlang CLI:

```sh
langlang generate --target rust --grammar png.peg --output src/png_parser.rs
```

Or from a `build.rs` script:

```rust
// build.rs
fn main() {
    langlang::codegen::rust("grammars/png.peg", "src/png_parser.rs")
        .binary(true)
        .derive(&["Debug", "Clone"])
        .generate()
        .unwrap();
}
```

### Rust-specific codegen considerations

**Proc macros vs build.rs.** Rust has two codegen mechanisms: procedural
macros (run at compile time, produce token streams) and `build.rs` scripts
(run before compilation, produce files). `build.rs` is the right choice
because:

- The grammar file is an external input, not Rust source
- The generated code can be inspected (`cargo expand` or just reading the
  output file)
- Build scripts can invoke the langlang CLI binary, avoiding the need to
  reimplement the grammar compiler in Rust (the Go CLI does the heavy lifting)

The flow: `build.rs` shells out to the `langlang` CLI (built from Go),
which reads the grammar and emits a `.rs` file. The `.rs` file is compiled
by `rustc` as part of the normal build. This is the same pattern that
`protoc` + `prost-build` uses for protobuf.

**No-std support.** For embedded and kernel use cases, the generated Rust
code should work without the standard library. Binary parsing is a common
need in no-std environments (parsing firmware headers, network packets,
sensor data). The generated code would use `core` types only — `&[u8]`,
fixed-size arrays, primitive integers. The `alloc` crate is needed only if
the grammar has repetitions (which produce `Vec<T>`).

**Error handling.** Rust's `Result<T, E>` maps naturally to parse results.
The generated decoder returns `Result<(T, &[u8]), ParseError>` where the
second element of the tuple is the remaining unconsumed input. This composes
well with Rust's `?` operator for chaining decoders.

## Comparison with Existing Tools

### nom (Rust parser combinator library)

nom is the standard Rust library for binary parsing. It uses runtime
combinators — you compose parsers from functions at compile time, but the
parsing logic is executed at runtime with function call overhead per combinator.

Langlang's Rust codegen would produce a single function per rule with the
combinators inlined. No function pointer dispatch, no closure allocation.
For deeply nested formats, the difference is measurable — each level of
nom combinator nesting adds a function call frame.

nom's advantage: runtime composability. You can build parsers dynamically.
Langlang's codegen is static — the grammar must be known at build time.

### kaitai struct

Kaitai Struct is a declarative binary format description language that
generates parsers in multiple languages (including Go and Rust). It's the
closest existing tool to what this exploration describes.

Differences:

- Kaitai uses a YAML-based schema language. Langlang uses PEG, which is
  more expressive (ordered choice, predicates, repetition with separators).
- Kaitai generates runtime-interpreted parsers that walk a schema tree.
  Langlang would generate compiled parsers with no schema interpretation
  overhead.
- Kaitai supports `instances` (lazily evaluated fields) and `enums` natively.
  Langlang would need to express these via PEG constructs or grammar
  extensions.
- Kaitai has a large library of existing format descriptions. Langlang would
  start from scratch.

### deku (Rust derive macro for binary)

deku uses Rust derive macros to generate binary parsers from annotated structs:

```rust
#[derive(DekuRead, DekuWrite)]
struct Header {
    #[deku(bytes = "4")]
    magic: [u8; 4],
    #[deku(endian = "big")]
    length: u32,
}
```

The struct IS the schema. Langlang inverts this: the grammar is the schema,
and the struct is derived (or mapped) from it. Deku's approach is simpler for
flat formats. Langlang's approach is more powerful for formats with conditional
structure, where PEG's ordered choice and predicates express constraints that
Rust attributes can't.

### protobuf / flatbuffers / cap'n proto

These define both a wire format and an IDL. Langlang would only define the
parser/serializer — the wire format is whatever the grammar describes. This
means langlang can parse existing formats (PNG, DNS, ELF) that these tools
can't target. But for new formats, the existing IDLs have mature ecosystems,
language-neutral schemas, and battle-tested implementations.

Langlang's niche for new formats would be formats with complex conditional
structure that TLV can't express efficiently, or formats where zero-copy
parsing of the exact wire layout matters more than schema evolution.

## Open Questions

1. **Value binding scope.** In `u32be -> length ... bytes(length)`, what is
   the scope of `length`? Options: the enclosing rule only (safest), the
   enclosing rule and its children (needed for most formats), or global
   within the parse (dangerous). Most binary formats bind a length in a
   container header and consume it in the body, which requires at least
   parent-to-child scope.

2. **Computed values.** Some formats need arithmetic on bound values:
   `bytes(length - 4)` (data length excluding a header). How much
   expression support should the grammar syntax have? Minimal (only `+`,
   `-`, `*`, bitwise AND) vs. none (the extraction codegen handles
   computation on the Go/Rust side). Minimal is more useful but adds a
   small expression evaluator to the compiler.

3. **Alignment and padding.** Many binary formats have alignment constraints
   (fields at 4-byte boundaries, padding bytes between fields). The grammar
   can express this with explicit padding rules (`Padding <- x"00"*`) but
   a dedicated `@align 4` directive would be cleaner.

4. **Bit-level parsing.** Some formats operate at the bit level (TCP flags,
   PNG filter types, compression headers). PEG operates at the byte level.
   Bit-level support requires either a sub-byte cursor or a separate
   bit-field syntax (`bits(3)` to consume 3 bits). This is a significant
   extension.

5. **Rust codegen bootstrapping.** The langlang CLI is written in Go. The
   Rust codegen would be a Go program that emits Rust source. This is a
   cross-language dependency. An alternative: rewrite the grammar compiler
   in Rust (the `langlang_lib` crate already has the VM and compiler — it
   just needs the codegen backend). The `build.rs` script would then be
   pure Rust with no Go dependency.

6. **Grammar-driven fuzzing.** A grammar that describes a binary format can
   also generate valid random inputs for that format. This is the inverse of
   parsing: walk the grammar, make random choices at each ordered choice, and
   emit the corresponding bytes. For formats with length-prefixed fields,
   generate the content first, then compute the length. This produces a
   grammar-aware fuzzer that's useful for testing both the generated parser
   and any system that consumes the format.

## Relationship to Other Explorations

This document extends the langlang exploration in a new direction — binary
data — while building on all previous explorations:

- **Extraction codegen** (`langlang-extract-plan.md`): the binary extraction
  fields (u32be, bytes) are additions to the FieldKind taxonomy
- **Arena strategies** (`langlang-arena-benchmark-plan.md`): binary fields
  are small and fixed-size, which favors the typed parallel arena (Strategy B)
  over the byte buffer
- **Divide-and-conquer** (`langlang-divide-and-conquer.md`): binary formats
  with TLV containers have decision junctions (type bytes, bracket bytes)
  that the scanner can detect
- **SIMD scanner** (`langlang-simd-scanner.md`): binary format scanners
  classify type bytes and length-prefix markers, same VPSHUFB technique
- **Bidirectional parsing** (`langlang-bidirectional-parsing.md`): less
  applicable to binary formats (most are forward-only), but formats like ZIP
  (central directory at end) could benefit

The Rust codegen dimension is new and applies retroactively to all
explorations — every optimization described in the previous documents
(arena-direct access, SIMD scanning, parallel parsing) has a Rust equivalent
that's potentially faster due to Rust's zero-cost abstractions, no-GC
execution model, and first-class SIMD support.

## Project Structure

```
go/
├── binary/                        # binary grammar extensions
│   ├── primitives.go              # u8, u16be, etc. as built-in rules
│   ├── primitives_test.go
│   ├── value_stack.go             # VM value stack for length binding
│   ├── value_stack_test.go
│   ├── directive.go               # @binary, @endian directive handling
│   └── directive_test.go
├── extract/
│   ├── binary_fields.go           # FieldU32BE, FieldBytes, etc.
│   ├── binary_emit.go             # encode + decode codegen for binary
│   └── binary_emit_test.go
└── cmd/
    └── langlang/
        └── generate.go            # --target go|rust flag

rust/
├── langlang_codegen/              # new crate: Rust source emission
│   ├── Cargo.toml
│   └── src/
│       ├── lib.rs
│       ├── emit.rs                # Rust source code emitter
│       ├── types.rs               # struct generation with lifetimes
│       ├── decode.rs              # decoder function generation
│       ├── encode.rs              # encoder function generation
│       └── simd.rs                # SIMD scanner generation
└── langlang_binary/               # new crate: binary grammar support
    ├── Cargo.toml
    └── src/
        ├── lib.rs
        ├── primitives.rs          # numeric type parsing
        └── value_stack.rs         # Rust VM value stack
```

---

## Architectural Note: Direct Codegen Without the VM

This document describes three new VM instructions (`IFixedWidth`,
`IBindValue`, `IConsumeBytes`) and a value stack for the VM interpreter.
With direct codegen, none of these are needed as runtime constructs.

The `u32be` grammar primitive becomes a direct call to
`binary.BigEndian.Uint32(input[pos:pos+4])` in Go, or
`u32::from_be_bytes(input[pos..pos+4].try_into()?)` in Rust. No instruction,
no interpreter dispatch.

The `-> length` value binding becomes a local variable in the generated
function:

```go
func parseChunk(input []byte, pos int) (PNGChunk, int, bool) {
    length := binary.BigEndian.Uint32(input[pos : pos+4])
    pos += 4
    chunkType := [4]byte(input[pos : pos+4])
    pos += 4
    data := input[pos : pos+int(length)]
    pos += int(length)
    crc := binary.BigEndian.Uint32(input[pos : pos+4])
    pos += 4
    return PNGChunk{length, chunkType, data, crc}, pos, true
}
```

The grammar's context-sensitivity (`bytes(length)` depending on a previously
parsed value) resolves at codegen time into variable scoping in the target
language. No value stack, no binding table, no runtime name resolution.
The `-> name` syntax in the grammar maps to a Go/Rust variable declaration;
the `bytes(name)` syntax maps to a slice expression using that variable.

This is substantially simpler and faster than the VM-based approach described
in the main document. The project structure changes accordingly:

- `binary/value_stack.go` is unnecessary — values are local variables
- `binary/primitives.go` becomes a set of emitter patterns, not VM
  instructions
- The Rust `langlang_binary/value_stack.rs` is unnecessary
- `langlang_codegen/decode.rs` emits Rust functions directly from the
  grammar AST, not from bytecode

The VM instructions described in this document (`IFixedWidth`, etc.) remain
useful as a specification of what the emitter should produce — they document
the semantics of binary primitives. But they are not executed at runtime.
