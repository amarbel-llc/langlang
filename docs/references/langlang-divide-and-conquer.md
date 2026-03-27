# Divide-and-Conquer Parsing via Decision Junction Scanning

## Concept

Certain bytes in a grammar can only appear at positions where the parser makes a
routing decision — entering a container, leaving a container, or separating
siblings. These bytes are **decision junctions**. If you can find them in a
lightweight pre-scan, you know the parse tree's branching skeleton before any PEG
parsing has occurred. The skeleton partitions the input into independent regions
that can be parsed in parallel, recursively, at every depth level.

This differs from bidirectional parsing (which splits at the top level only) and
from anchor scanning (which looks for rule-leading patterns). Decision junction
scanning exploits the recursive nesting structure of the grammar to achieve
N-way parallelism at every depth, with the splitting points derived mechanically
from the grammar's literal delimiters.

## JSON as the Canonical Example

```peg
Value  <- Object / Array / String / Number / 'true' / 'false' / 'null'
Object <- '{' (Member (',' Member)*)? '}'
Member <- String ':' Value
Array  <- '[' (Value (',' Value)*)? ']'
String <- '"' Char* '"'
```

The bytes `{`, `}`, `[`, `]`, `,`, `:` have a special property: they can only
appear at structural positions *outside of strings*. Inside a string, these
bytes are literal content. The grammar makes this explicit — `String` consumes
everything between `"` delimiters, and the structural bytes only appear as
`LiteralNode` children of `SequenceNode` in the Object, Array, and Member rules.

A single `O(n)` scan with 1 bit of state (inside-string toggle) finds every
structural byte and its depth:

```
input:   {"name":"alice","items":[1,2,{"x":3}]}

structural skeleton:
  { at 0   depth 0  (open Object)
  : at 6   depth 1  (pair separator)
  , at 14  depth 1  (sibling separator)
  : at 21  depth 1  (pair separator)
  [ at 23  depth 1  (open Array)
  , at 25  depth 2  (sibling separator)
  , at 27  depth 2  (sibling separator)
  { at 28  depth 2  (open Object)
  : at 31  depth 3  (pair separator)
  } at 37  depth 2  (close Object)
  ] at 38  depth 1  (close Array)
  } at 40  depth 0  (close Object)
```

This skeleton is the parse tree's shape. The actual PEG parsing of each leaf
region (string content, numbers, booleans) can happen independently.

## Decision Junction Classification

A **decision junction** is a `LiteralNode` in the grammar AST that satisfies
all of the following:

1. **Positional**: it is a direct child of a `SequenceNode`, appearing as a
   delimiter or separator rather than as content.

2. **Unambiguous in context**: the byte(s) cannot appear as matched content
   within sibling rules at the same sequence level, unless those sibling rules
   define a quoting context (like String) that the scanner can track.

3. **Structurally significant**: it marks one of:
   - **Open**: enters a new nesting level (increases depth)
   - **Close**: exits a nesting level (decreases depth)
   - **Separator**: divides siblings at the same depth

### Formal extraction from the grammar AST

Walk each `DefinitionNode.Expr`. For each `SequenceNode`, classify its
`LiteralNode` children:

```go
type JunctionKind int

const (
    JunctionOpen      JunctionKind = iota  // first literal in a sequence containing a repetition
    JunctionClose                          // last literal in a sequence containing a repetition
    JunctionSeparator                      // literal inside a repetition's inner sequence
)

type Junction struct {
    Bytes    []byte         // the literal bytes (e.g., '{', ',')
    Kind     JunctionKind
    Rule     string         // which rule this junction belongs to
    Depth    int            // nesting depth relative to the grammar root
    PairWith *Junction      // for Open, points to corresponding Close
}
```

For the JSON grammar, the analysis produces:

| Literal | Kind      | Rule   | Depth context                       |
|---------|-----------|--------|-------------------------------------|
| `{`     | Open      | Object | Paired with `}`                     |
| `}`     | Close     | Object | Paired with `{`                     |
| `[`     | Open      | Array  | Paired with `]`                     |
| `]`     | Close     | Array  | Paired with `[`                     |
| `,`     | Separator | Object | Separates Member siblings           |
| `,`     | Separator | Array  | Separates Value siblings            |
| `:`     | Separator | Member | Separates String key from Value     |
| `"`     | (context) | String | Toggles quoting — not a junction    |

The `"` byte is not a junction but a **context toggle**. The scanner must
track it to distinguish structural junctions from string-interior bytes.

### Quoting contexts

A **quoting context** is a rule where a delimiter pair (like `"..."` or
`` `...` ``) causes all interior bytes — including bytes that would otherwise
be junctions — to lose their structural meaning.

The codegen identifies quoting contexts by finding rules whose body is a
sequence of the form:

```
QuoteRule <- OPEN_LITERAL  body*  CLOSE_LITERAL
```

where `body` is a negation or wildcard that consumes arbitrary bytes including
junction bytes. The `String` rule in JSON fits this pattern: `'"' Char* '"'`
where `Char` matches anything except `"`.

The scanner tracks quoting state alongside depth. For JSON, this is a single
bit. For grammars with multiple quoting contexts (e.g., single-quoted strings,
double-quoted strings, backtick strings), it's a small enum.

### Escape handling

Inside a quoting context, the close delimiter may be escaped (e.g., `\"` inside
a JSON string). The grammar encodes this — JSON has `Escape <- '\\' ["\\/bfnrt]`
which shows that `\` followed by `"` is not a string terminator.

The scanner needs to recognize the escape prefix. The codegen extracts this from
the grammar: if the quoting context's body includes a rule whose first element
is a literal `\` (or another escape character), that literal prevents the
following byte from being interpreted as a close delimiter.

For JSON, the scanner logic becomes:

```go
if inString {
    if b == '\\' {
        i++  // skip the escaped byte
        continue
    }
    if b == '"' {
        inString = false
    }
    continue
}
if b == '"' {
    inString = true
    continue
}
// b is a structural byte — record as junction
```

## The Scanner

The generated scanner is a single-pass, `O(n)` function with minimal state.
Its output is a flat list of junction positions with depth and kind annotations.

```go
type junctionHit struct {
    pos   int32
    depth int16
    kind  uint8   // open=0, close=1, separator=2
    rule  uint8   // which junction literal matched
}

func scanJunctions(input []byte) []junctionHit {
    var (
        hits     []junctionHit
        depth    int16
        inString bool
    )
    for i := 0; i < len(input); i++ {
        b := input[i]

        // Quoting context tracking
        if inString {
            if b == '\\' {
                i++
                continue
            }
            if b == '"' {
                inString = false
            }
            continue
        }
        if b == '"' {
            inString = true
            continue
        }

        // Junction detection
        switch b {
        case '{', '[':
            hits = append(hits, junctionHit{
                pos: int32(i), depth: depth, kind: 0,
            })
            depth++
        case '}', ']':
            depth--
            hits = append(hits, junctionHit{
                pos: int32(i), depth: depth, kind: 1,
            })
        case ',', ':':
            hits = append(hits, junctionHit{
                pos: int32(i), depth: depth, kind: 2,
            })
        }
    }
    return hits
}
```

This scanner is grammar-generated. The `switch` cases, quoting context logic,
and escape handling are all derived from the grammar AST analysis. A different
grammar produces a different scanner.

### Performance characteristics

The scanner processes one byte per iteration. No backtracking, no VM, no
stack. On modern hardware, this is limited by memory bandwidth — roughly 4-8
GB/s on a single core, or ~8 ns per byte for a cache-resident 1 MB input.

For comparison, the full PEG VM processes the same input at ~50-200 ns per byte
(depending on grammar complexity, backtracking, and capture overhead). The
scanner is 10-25x cheaper than parsing.

The scanner output (the junction list) is typically small. A 1 MB JSON document
with 10K structural elements produces ~30K junctions (opens + closes +
separators). At 8 bytes per junction hit, that's ~240 KB — a small fraction of
the input.

SIMD (Single Instruction, Multiple Data) acceleration is possible: classify
16/32/64 bytes simultaneously using vector comparisons, then process only the
bytes that matched any junction or quote character. This is what simdjson does
for JSON specifically. The grammar-derived scanner generalizes this: the set of
interesting bytes is computed from the grammar, and the SIMD mask is generated
accordingly.

## Partitioning

The junction list defines a hierarchical partition of the input.

### Building the partition tree

```go
type partition struct {
    start    int32           // byte offset in input (exclusive of opening delimiter)
    end      int32           // byte offset in input (exclusive of closing delimiter)
    kind     partitionKind   // object, array, etc.
    children []partition     // sub-partitions (siblings at this depth)
}

type partitionKind uint8

const (
    partObject partitionKind = iota
    partArray
    partMember
    // ... grammar-specific
)
```

Construction walks the junction list and builds a tree:

1. An Open junction pushes a new partition onto a stack
2. A Separator junction closes the current sibling region and opens the next
3. A Close junction pops the partition stack, recording the closed region

This is `O(j)` where `j` is the number of junctions — much smaller than `n`.

For the JSON example:

```
partition(Object, 0..40)
├── partition(Member, 1..14)   "name":"alice"
│   ├── key region: 1..6      "name"
│   └── value region: 8..14   "alice"
├── partition(Member, 15..40)  "items":[1,2,{"x":3}]
│   ├── key region: 15..21    "items"
│   └── value region: 23..39
│       └── partition(Array, 23..38)
│           ├── element: 24..25    1
│           ├── element: 26..27    2
│           └── element: 28..37
│               └── partition(Object, 28..37)
│                   └── partition(Member, 29..36) "x":3
```

### Independence property

**Siblings at the same depth are parse-independent.** The PEG grammar processes
them with the same rule, starting at each sibling's start position. No sibling's
parse result affects another's. This is the property that enables parallelism.

**Children of different subtrees are also independent.** Parsing the `"name"`
member has no effect on parsing the array `[1,2,{"x":3}]`. They don't share
any parser state.

The only dependency is structural: you can't determine a sibling's start
position without having scanned past the previous sibling's content (to find the
separator). But the scanner already did this in the pre-pass. The partition tree
encodes all start/end positions, so the parallel parsers don't need to discover
them.

## Slab Pre-Allocation from the Skeleton

### The key insight

The scanner tells you the exact count of every container type before any parsing
begins. You know how many objects, arrays, members, and array elements exist
because you counted the junctions. That's enough to pre-allocate the entire
output structure.

For the running example `{"name":"alice","items":[1,2,{"x":3}]}`, before any
PEG parsing:

- 2 objects (two `{` junctions)
- 1 array (one `[` junction)
- 3 object members (count `:` at each depth)
- 3 array elements (count `,` + 1 between `[` and `]`)

That's the exact shape of the typed output.

### Tagged union slots

The grammar's `Value` rule is a choice of alternatives with different natural
sizes. Rather than maintaining separate allocations per alternative, accept
over-allocation to the largest variant and use a tagged union approach:

| Alternative | Payload needed                                     |
|-------------|----------------------------------------------------|
| Object      | index into members region + member count (8 bytes) |
| Array       | index into values region + element count (8 bytes) |
| String      | start offset + end offset into input (8 bytes)     |
| Number      | start offset + end offset into input (8 bytes)     |
| Boolean     | 1 byte                                             |
| Null        | 0 bytes                                            |

The maximum payload is 8 bytes. Add a 1-byte tag. Align to 16 bytes for cache
line friendliness. Every value slot is 16 bytes regardless of what's inside:

```go
type ValueSlot struct {
    tag  uint8
    _    [7]byte   // padding
    data [8]byte   // payload, interpreted based on tag
}

const (
    tagObject  uint8 = iota
    tagArray
    tagString
    tagNumber
    tagTrue
    tagFalse
    tagNull
)
```

Reading a value is a switch on the tag with direct byte access to the payload:

```go
func readValue(slot *ValueSlot, input []byte) JSONValue {
    switch slot.tag {
    case tagString:
        start := *(*int32)(unsafe.Pointer(&slot.data[0]))
        end   := *(*int32)(unsafe.Pointer(&slot.data[4]))
        return JSONValue{String: ptrTo(string(input[start:end]))}
    case tagNumber:
        start := *(*int32)(unsafe.Pointer(&slot.data[0]))
        end   := *(*int32)(unsafe.Pointer(&slot.data[4]))
        return JSONValue{Number: ptrTo(string(input[start:end]))}
    case tagObject:
        idx   := *(*int32)(unsafe.Pointer(&slot.data[0]))
        count := *(*int32)(unsafe.Pointer(&slot.data[4]))
        return JSONValue{Object: &objectRef{idx, count}}
    // ...
    }
}
```

Or without `unsafe`, using `encoding/binary.LittleEndian`, at the cost of
function call overhead per field.

This extends to every container type. Members also get a uniform slot:

```go
type MemberSlot struct {
    keyStart   int32    // offset into input
    keyEnd     int32
    valueIndex int32    // index into the values slab
    _          [4]byte  // pad to 16 bytes
}
```

### The single-slab model

Since both slot types are the same width (16 bytes), they don't even need
separate backing arrays. They can be interleaved in a single allocation:

```go
type Slab struct {
    buf   []byte  // (totalSlots) * slotStride bytes
    input []byte  // borrowed
}
```

The partition tree assigns each partition a contiguous range of slot indices in
the single buffer. The slot's tag byte tells you what's in it — value, member,
whatever. The layout in memory mirrors the document structure because the
partition builder assigns indices in document order.

This is very close to how an ECS (Entity Component System) works in game
engines. One big flat buffer, fixed-stride slots, tag-dispatched access. The
properties that make ECS fast are the same ones that help here: linear memory
access patterns, predictable prefetching, no pointer chasing, no GC (Garbage
Collector) overhead from individual allocations.

### Allocation from the skeleton

The allocation happens in one shot, right after the scan, before parsing starts:

```go
func allocateFromSkeleton(hits []junctionHit) *Slab {
    var nObjects, nArrays, nMembers, nElements int
    for _, h := range hits {
        switch {
        case h.kind == junctionOpen && h.rule == ruleObject:
            nObjects++
        case h.kind == junctionOpen && h.rule == ruleArray:
            nArrays++
        case h.kind == junctionSeparator && h.rule == ruleMember:
            nMembers++
        case h.kind == junctionSeparator && h.rule == ruleArray:
            nElements++
        }
    }
    // Each object has at least one member (the first, before any comma)
    nMembers += nObjects
    // Each array has at least one element
    nElements += nArrays

    totalSlots := nObjects + nArrays + nMembers + nElements
    return &Slab{
        buf: make([]byte, totalSlots*slotStride),
    }
}
```

One `make` call. No arena growth, no `append` reallocation during parsing, no
GC pressure from abandoned speculative allocations.

### Slot assignment during partitioning

The partition builder (between scan and parse) walks the junction hits and
assigns contiguous slot ranges to each partition:

```go
type partitionTask struct {
    inputStart int32
    inputEnd   int32
    slotIndex  int32   // where in the pre-allocated slab to write
    slotCount  int32   // how many slots this partition fills
}
```

For the JSON example:

```
partition(Object, bytes 0..40)  → objects[0], members[0..1]
  partition(Member, bytes 1..14)  → members[0], values[0]
    leaf: "alice" → values[0] = String
  partition(Member, bytes 15..40) → members[1], values[1]
    partition(Array, bytes 23..38) → arrays[0], values[2..4]
      leaf: 1 → values[2] = Number
      leaf: 2 → values[3] = Number
      partition(Object, bytes 28..37) → objects[1], members[2], values[5]
        partition(Member, bytes 29..36) → members[2], values[5]
          leaf: 3 → values[5] = Number
```

Each goroutine gets a task with its slot range. It parses its region and writes
directly into the pre-allocated slab at its assigned indices. No locks — the
index ranges don't overlap.

### What the goroutine write path looks like

```go
func parseLeafValue(slab *Slab, input []byte, slotIndex int, region []byte, regionStart int) {
    slot := slab.buf[slotIndex*slotStride : (slotIndex+1)*slotStride]

    // The PEG parser figures out which alternative matched.
    // For a number:
    slot[0] = tagNumber
    putI32(slot, 8, int32(regionStart))
    putI32(slot, 12, int32(regionStart+len(region)))
}
```

No allocation. No arena. No tree. No extraction pass. The goroutine writes
directly into its assigned 16-byte slot in the pre-allocated slab. The write is
two `int32` stores and one `uint8` store. That's it.

### Leaf classification without the PEG VM

The goroutine doesn't even need a full PEG VM for most leaf partitions. A leaf
partition is the region between junctions — the content of a string, a number
literal, a boolean keyword. Recognizing which Value alternative matched is a
one-byte check:

```go
switch region[0] {
case '"': // string
case 't': // true
case 'f': // false
case 'n': // null
case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': // number
default:  // error
}
```

You don't need the PEG VM at all for leaf classification. The junction scanner
already told you the boundaries. The leaf parser is a trivial switch. The PEG VM
is only needed for validation (confirming that the content between junctions is
well-formed according to the grammar's terminal rules) — and even that could be
deferred or skipped in a "fast path" mode that trusts well-formed input.

### Pipeline collapse

This collapses the entire parsing pipeline:

```
Traditional:    input → PEG VM → generic tree → extraction walk → typed output
Level 0:        input → PEG VM → arena tree → arena-direct extraction → typed output
D&C + slab:     input → junction scan → slab alloc → parallel slot writes → done
```

The slab *is* the output. There's no intermediate representation. The junction
scan produces the shape, the slab allocation reserves the space, and the
parallel writers fill it. The PEG VM becomes optional — it's a validation tool,
not a structural parser, because the scanner already determined the structure.

### Over-allocation tradeoff

The tradeoff being accepted — over-allocation to the largest variant — costs
`padding * slot_count` bytes. For JSON, if the average value is 6 bytes of
payload but every slot is 16 bytes, roughly 60% of the slab space is padding.
For a 1 MB input with 10K values, that's ~160 KB of slots with ~100 KB wasted.
At the scale of a 1 MB input, 100 KB of waste is irrelevant. Even at 100 MB
inputs, 10 MB of waste is usually fine — it's a constant factor, not a
complexity change.

Where it starts to matter is if the slot size variance is extreme. If one
alternative requires 128 bytes (a large struct) and another requires 1 byte
(a boolean), padding to 128 bytes per slot is wasteful. But this is a property
of the grammar — the codegen can compute the max slot size and warn if the
waste ratio exceeds a threshold.

For dodder's use cases (organize text, box format), the value alternatives are
likely similar in size (all text-based, all storing byte offsets into the input).
The waste ratio should be low.

### String handling

For strings specifically, the scanner identifies string boundaries (the `"`
toggles), so you know where every string starts and ends. But you don't know
the decoded content yet — escape sequences like `\n` mean the decoded string is
shorter than the raw bytes. Two options:

**Option 1: raw byte refs.** Store `StringRef{start, end int32}` in the
pre-allocated slot. The ref points into the original input. Decoding happens
lazily when the consumer accesses the string. No allocation for strings that are
never read.

**Option 2: pre-allocated decode buffer.** Pre-allocate a string decode buffer
sized to the total string byte count (which is bounded by the input size). Each
goroutine writes decoded strings into its assigned region of the buffer. This is
the zero-copy path — the final typed struct contains Go strings backed by the
decode buffer.

Option 1 is simpler and better for most use cases. The organize text format in
dodder has few escape sequences, so raw byte refs are almost always equal to the
decoded content.

### Grammars with optional elements

For grammars where the junction count doesn't exactly predict the output count —
for example, `Member <- String ':' Value Annotation?` where Annotation is
optional — the scanner knows how many Members there are but not how many
Annotations. The pre-allocation over-allocates Annotation slots (one per
Member) and some go unused. This is wasted memory but not incorrect — unused
slots are zero-valued. The tag byte distinguishes filled slots from unfilled
ones.

### Error handling

The pre-allocation model changes the error path. If a partition fails to parse
(malformed content), the slot is pre-allocated but unfilled. The document struct
needs a way to represent this — either a validity bit per slot (the zero-valued
tag already serves this purpose), or a separate error list that references slot
indices. The partition tree already knows which slots map to which byte ranges,
so error messages can pinpoint the exact input region.

## Parallel Execution

### Strategy: recursive fan-out

At each partition node with multiple children, fan out goroutines for the
children. Each goroutine receives its input slice and the slot index range
where it should write results.

```go
func parsePartition(slab *Slab, input []byte, part partition) error {
    if len(part.children) == 0 {
        // Leaf partition: write directly into assigned slot
        return parseLeaf(slab, input, part)
    }

    var wg sync.WaitGroup
    errs := make([]error, len(part.children))

    for i, child := range part.children {
        wg.Add(1)
        go func(i int, child partition) {
            defer wg.Done()
            errs[i] = parsePartition(slab, input, child)
        }(i, child)
    }
    wg.Wait()

    return firstError(errs)
}
```

### Work-stealing refinement

Naive goroutine-per-child is wasteful for small partitions. A JSON array of
1000 small numbers doesn't benefit from 1000 goroutines — the spawn overhead
exceeds the parse time per element.

Refinement: only fan out when a partition's byte span exceeds a threshold
(e.g., 4 KB). Below that, parse sequentially into the slab. The threshold is
tunable and grammar-dependent.

```go
const parallelThreshold = 4096  // bytes

func parsePartition(slab *Slab, input []byte, part partition) error {
    span := int(part.end - part.start)

    if span < parallelThreshold || len(part.children) <= 1 {
        return parseSequential(slab, input, part)
    }

    // Fan out
    // ...
}
```

A more sophisticated approach: use a work-stealing pool (`sync.Pool` of parser
instances + a task queue). The scanner generates the partition tree, the
scheduler enqueues large partitions, and a fixed pool of worker goroutines
(one per CPU core) dequeues and parses.

### Depth-limited parallelism

For deeply nested documents, restrict parallelism to the top K depth levels.
Below depth K, parse sequentially. This bounds goroutine count to the fan-out
of the top K levels.

For JSON: depth 0 is the root object/array, depth 1 is its immediate children.
Parallelizing just depth 0 and 1 captures most of the available concurrency
(the top-level members of a large JSON object or elements of a large JSON
array).

## Correctness

### The scanner must agree with the parser

The scanner's junction hits must correspond exactly to the positions where the
PEG parser would encounter those literals. If the scanner misidentifies a
byte as a junction (or misses one), the partitioning is wrong and the parallel
parses will fail or produce incorrect results.

This correctness property holds if and only if:

1. **The quoting context is correctly tracked.** Every byte that suppresses
   junction detection inside the scanner must correspond to a grammar rule
   that consumes those bytes without treating them as structural.

2. **Junctions are context-free outside of quotes.** A `{` outside of any
   quoting context always means "open object," never something else. This
   is a property of the grammar, not the scanner — the codegen must verify it.

3. **No junction bytes appear in non-quoted terminal rules.** If a grammar has
   `Identifier <- [a-zA-Z{}]+` (allowing `{` inside identifiers), then `{` is
   not a valid junction because it can appear as content. The codegen's analysis
   must check this by examining the character classes and negation predicates
   of all terminal rules reachable from non-quoting contexts.

### Validation

The codegen performs reachability analysis on the grammar AST:

```
for each candidate junction byte B:
    for each terminal rule T reachable from a non-quoting context:
        if T can match byte B:
            B is NOT a valid junction — disqualify it
```

A terminal rule can match byte B if:
- It contains a `CharsetNode` whose range includes B
- It contains an `AnyNode` (`.`)
- It contains a `RangeNode` whose range includes B
- It contains a `ClassNode` whose set includes B

The `AnyNode` case is common (e.g., `Char <- Escape / (!'"' .)`). But this
rule is inside the String quoting context, so the scanner won't be looking
for junctions there. The reachability analysis must exclude rules that are
only reachable through quoting contexts.

This is a fixed-point computation:

```go
func reachableTerminals(grammar *GrammarNode, quoteRules map[string]bool) map[string]bool {
    // BFS/DFS from the start rule, excluding edges into quoteRules
    // Returns the set of terminal rules reachable from non-quoting paths
}
```

If a junction byte survives this analysis, the scanner is correct.

## Grammars Where This Works Well

### JSON

The canonical case. Six junction bytes (`{`, `}`, `[`, `]`, `,`, `:`), one
quoting context (`"`), one escape prefix (`\`). The scanner is tiny and the
partitioning is clean.

### TOML

Tables (`[table]`, `[[array-of-tables]]`), key-value pairs (`key = value`
separated by newlines), string quoting (basic `"..."`, literal `'...'`,
multi-line `"""..."""`, `'''...'''`). Junctions: `[`, `]`, `=`, `\n` (as
separator between key-value pairs). More complex quoting context (four string
types) but still a small state machine.

### Organize text (dodder)

Heading-delimited sections. The heading prefix (e.g., `-` or `#` at start of
line) is a junction. Section bodies are independent. The quoting context is
the box format regions (delimited by specific markers). Directly applicable.

### XML / HTML

`<` and `>` are junctions. Quoting contexts: attribute values (`"..."`,
`'...'`), CDATA sections (`<![CDATA[...]]>`), comments (`<!--...-->`).
More complex but mechanically derivable.

### S-expressions

`(` and `)` are junctions. String quoting: `"..."`. Minimal state. Very clean.

## Grammars Where This Doesn't Work

### Expression grammars

`Expr <- Term ('+' Term)*` — the `+` is a separator, but Term can be
arbitrarily complex (including parenthesized sub-expressions). The junction
analysis works for the outer `+`, but the parallelism is limited because
the depth is shallow (typical expressions don't have thousands of top-level
`+` operations).

### Natural language grammars

No delimiters, no quoting contexts. Sentences and words don't have
structurally unambiguous boundaries.

### Grammars with ambiguous junction bytes

If `{` can appear both as a structural delimiter and as content (e.g., a
template language where `{variable}` is a substitution and `{` in a code
block is a different construct), the junction analysis fails. The scanner
can't distinguish the two uses without full parsing.

## Benchmark Considerations

### Metrics specific to this strategy

1. **Scan time** (`ns/byte`): how fast the generated scanner processes the
   input. Baseline comparison: memchr speed (~0.5 ns/byte with SIMD, ~2
   ns/byte scalar).

2. **Partition + allocation overhead** (`ns/junction`): time to build the
   partition tree from the junction list and allocate the slab.

3. **Parallel speedup** as a function of:
   - Input size (crossover point where parallelism amortizes overhead)
   - Number of top-level siblings (fan-out at depth 0 and 1)
   - Number of CPU cores
   - Partition size distribution (many small siblings vs few large ones)

4. **Total wall time** = scan time + partition time + alloc time +
   max(leaf parse times). Compare against sequential total = parse time +
   extract time.

5. **Allocation count**: should be O(1) (one slab `make` call) regardless
   of input size or structure. Compare against baseline which is O(n) in
   the number of tree nodes.

6. **Memory overhead ratio**: slab bytes allocated / useful payload bytes.
   Measures the cost of the tagged union over-allocation tradeoff.

### Expected results

For a 1 MB JSON document with ~10K structural elements:
- Scanner: ~250 μs (1 MB at ~4 bytes/ns)
- Partition tree + slab alloc: ~10 μs (30K junctions, one `make` call)
- Parallel slot writes on 8 cores: ~parse_time/6 (accounting for load
  imbalance and overhead)
- Sequential parsing: ~parse_time

Speedup approaches core count for documents with many similarly-sized
top-level elements (large JSON arrays). Degrades toward 1x for documents with
one dominant element (a single large nested object).

## Open Questions

1. **Multi-byte junctions.** JSON's junctions are all single bytes. TOML's
   `[[` (array-of-tables open) is two bytes. The scanner needs to handle
   multi-byte literals in the switch statement. Not fundamentally hard, but
   the SIMD acceleration path is trickier for multi-byte patterns.

2. **Overlapping junction bytes.** If `{` is an Open junction and `{#` is
   a different Open junction (e.g., in a template language), the scanner
   needs longest-match semantics. The generated scanner would need to look
   ahead one byte on `{` before committing. This is still `O(n)` but adds
   branching.

3. **Depth limit for parallelism.** How deep should the recursive fan-out go?
   The optimal depth depends on the core count and the partition size
   distribution. A static depth limit (e.g., depth ≤ 2) is simple. An
   adaptive limit based on estimated partition sizes (from the junction
   positions) is better but more complex.

4. **Error recovery across partitions.** If one partition fails to parse, the
   error position is in that partition's byte range. The error message should
   reference the original input positions, not the partition's local offsets.
   The partition tree preserves the original offsets, so this is a
   straightforward translation.

5. **Interaction with langlang's error recovery.** Langlang supports error
   recovery labels (e.g., `']'^arrayClose`). If a partition parse fails, the
   error recovery may try to consume bytes beyond the partition boundary. The
   parallel parser must constrain recovery to within the partition's byte
   range. This may require a small VM modification: a "hard end" cursor limit
   that recovery cannot exceed.

6. **SIMD scanner generation.** The scanner's inner loop is a classification
   of bytes into junction/quote/escape/other. This maps directly to SIMD
   shuffle-based classification (as in simdjson). The codegen could emit
   SIMD intrinsics for the scanner when targeting x86-64 or ARM64 with
   appropriate build tags. This is a performance optimization, not a
   correctness requirement, and should be explored only after the scalar
   scanner is working.

7. **Slab slot stride selection.** The 16-byte stride chosen in the JSON
   example is grammar-specific. The codegen computes the maximum payload
   across all alternatives in every choice rule, adds the tag byte, and
   rounds up to the nearest power of two for alignment. If the maximum
   payload is large (e.g., a grammar with one alternative requiring 120
   bytes), the waste ratio for small alternatives may be unacceptable. The
   codegen should emit a warning when the waste ratio exceeds a configurable
   threshold (e.g., 4x) and suggest splitting the slab into per-rule-kind
   regions as a fallback.

8. **Validation deferral.** The slab model allows parsing to succeed based
   solely on the junction skeleton, without validating that leaf content
   (strings, numbers, booleans) is well-formed according to the grammar's
   terminal rules. Should validation be mandatory, optional (a "strict mode"
   flag), or lazy (validate on first access)? Lazy validation composes well
   with the byte-ref string model (option 1) — validate and decode only the
   strings the consumer actually reads.

## Project Structure

```
go/
├── junction/                      # grammar analysis for junctions
│   ├── analyze.go                 # AST walk → ScannerSpec
│   ├── analyze_test.go
│   ├── reachability.go            # terminal reachability for validation
│   └── reachability_test.go
├── scanner/                       # generated scanner support
│   ├── emit.go                    # ScannerSpec → Go scanner function
│   ├── emit_test.go
│   ├── partition.go               # junction hits → partition tree
│   └── partition_test.go
├── slab/                          # pre-allocated output structure
│   ├── slab.go                    # Slab type, slot read/write helpers
│   ├── slab_test.go
│   ├── emit.go                    # codegen for slot types and readers
│   └── emit_test.go
├── parallel/                      # orchestration
│   ├── orchestrator.go            # recursive fan-out over partitions
│   ├── orchestrator_test.go
│   └── scheduler.go               # work-stealing pool for large inputs
└── cmd/
    └── langlang/
        └── parallel.go            # CLI flag: --parallel for D&C mode
```

## Relationship to Other Explorations

This document is one of three independent explorations of parallelism in
langlang parsing:

1. **Typed arena strategies** (`langlang-arena-benchmark-plan.md`): allocation
   efficiency for sequential parsing. The slab model described here supersedes
   the arena for grammars with decision junctions — the slab is pre-allocated
   from the skeleton rather than grown during parsing. For grammars without
   junctions, the arena strategies still apply.

2. **Bidirectional parsing** (`langlang-bidirectional-parsing.md`): 2-way
   parallelism via grammar reversal. Complementary — bidirectional targets
   grammars without decision junctions (expression grammars); D&C targets
   grammars with them (data formats). A grammar could use both: D&C for
   the container structure, bidirectional for the leaf expressions within
   containers.

3. **This document**: N-way parallelism via decision junction scanning with
   pre-allocated slab output. The most broadly applicable strategy for data
   format grammars, and the one that most aggressively eliminates intermediate
   representations — the scanner produces the structure, the slab holds the
   output, and the PEG VM becomes an optional validation layer rather than the
   primary structural parser.

---

## Architectural Note: Direct Codegen Without the VM

The D&C strategy already reduces the VM's role — the scanner finds the
structure, the partitioner determines the tree shape, and the VM only parses
leaf regions. Direct codegen takes this further: the leaf parsers are native
functions emitted from the grammar AST, not VM-interpreted bytecode.

Each leaf partition's parse becomes a direct function call:

```go
// Instead of: vm.MatchRule(input[partition.start:partition.end], ruleAddr)
// Direct codegen:
result := parseValue(input[partition.start:partition.end], 0)
```

The per-goroutine cost drops because there's no VM instance to construct,
no bytecode to load, no instruction dispatch. Each goroutine calls a native
function and returns a typed result. The goroutine spawn + function call
overhead is the only per-partition cost beyond the actual byte matching.

The scanner itself is unaffected — it's already a generated function, not a
VM-interpreted program. The partitioner is also unaffected.

Open question 5 (error recovery across partitions) becomes simpler with
direct codegen. Instead of modifying the VM to respect a "hard end" cursor
limit, the emitter generates parse functions that take `(input []byte, pos,
limit int)` and check `pos < limit` at every advance. The limit is the
partition boundary. No VM modification needed.
