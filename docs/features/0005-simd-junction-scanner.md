---
date: 2026-03-27
promotion-criteria: AVX2 scanner produces identical junction lists to scalar
  scanner for all test inputs, and benchmarks show \>=8x throughput improvement
  on 1 MB inputs.
status: proposed
---

# SIMD Junction Scanner

## Problem Statement

The divide-and-conquer scanner (FDR-0003) classifies each input byte as a
junction, quote toggle, escape prefix, or uninteresting. The scalar
implementation processes one byte per iteration with branch-heavy control flow
(\~2-4 ns/byte). For large inputs, this scan becomes a measurable fraction of
total parse time. SIMD instructions can classify 32 bytes (AVX2) or 64 bytes
(AVX-512) per cycle using vectorized table lookup, achieving 8-32x throughput
improvement.

## Interface

**Codegen (grammar to SIMD tables):** The codegen builds two 16-byte lookup
tables from the grammar's `ScannerSpec` -- one indexed by low nibble, one by
high nibble. Different grammars produce different table contents; the SIMD
skeleton is identical.

**Assembly generation:** An avo (`github.com/mmcloughlin/avo`) generator program
is emitted at `go generate` time. A second `go generate` pass runs it to produce
`.s` assembly.

``` go
//go:generate langlang extract -grammar=json.peg
// produces: json_extract.go, gen_scanner_asm.go, scanner_generic.go

//go:generate go run gen_scanner_asm.go -out scanner_amd64.s -stubs scanner_stub_amd64.go
// produces: scanner_amd64.s, scanner_stub_amd64.go
```

**Runtime dispatch:**

``` go
func scanJunctions(input []byte) []junctionHit {
    if cpu.X86.HasAVX2 {
        return scanJunctionsAVX2(input)
    }
    return scanJunctionsScalar(input)
}
```

**Build tags:** `scanner_generic.go` (`!amd64`), `scanner_amd64.s` + stubs
(`amd64`). ARM64 and other platforms use the scalar fallback automatically.

## Examples

The VPSHUFB classification for JSON:

  Byte   Hex    Low nibble   Category
  ------ ------ ------------ --------------
  `{`    0x7B   0xB          Open
  `}`    0x7D   0xD          Close
  `[`    0x5B   0xB          Open
  `]`    0x5D   0xD          Close
  `"`    0x22   0x2          Quote toggle

Two-pass classification (low nibble + high nibble + AND) resolves collisions
where different bytes share a nibble.

**Expected throughput:** Scalar \~2-4 ns/byte, AVX2 \~0.15-0.3 ns/byte, AVX-512
\~0.08-0.15 ns/byte.

## Limitations

- x86-64 only. No mature avo equivalent for ARM NEON/SVE exists in Go. ARM64
  users get the scalar fallback.
- Targets one function only (junction scanner). VM dispatch and arena access are
  not SIMD-accelerated.
- Avo's API is not stable; pin the dependency version. If avo breaks, delete
  `.s` and stubs -- the scalar fallback activates via build tags.
- Maximum 8 distinct byte categories per grammar (VPSHUFB bit budget). Grammars
  exceeding this fall back to scalar.
- Quoting state tracking is scalar post-processing of SIMD candidates (Approach
  B). Fully vectorized quoting (Approach A, as in simdjson) is deferred.

## More Information

- Full design with VPSHUFB classification algorithm, avo generator structure,
  and quoting approaches:
  [langlang-simd-scanner.md](../references/langlang-simd-scanner.md)
- Depends on: [FDR-0003 Divide-and-Conquer
  Parsing](0003-divide-and-conquer-parsing.md) (the scanner this accelerates)
- Prior art: simdjson's structural character classification
