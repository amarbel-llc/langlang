package junction

// JunctionKind classifies a structural delimiter's role.
type JunctionKind uint8

const (
	JunctionOpen      JunctionKind = iota // enters a nesting level (e.g. '{', '[')
	JunctionClose                         // exits a nesting level (e.g. '}', ']')
	JunctionSeparator                     // divides siblings at the same depth (e.g. ',', ':')
)

// JunctionByte maps a single byte to its structural role.
type JunctionByte struct {
	Byte byte
	Kind JunctionKind
}

// JunctionSequence is a multi-byte structural delimiter. The first byte
// triggers candidate evaluation; the scanner peeks ahead to confirm or
// reject the match. Longest match wins over shorter sequences and
// single-byte junctions sharing the same first byte.
//
// NOTE: The current implementation uses lookahead (random access) to
// resolve matches. Streaming input support would require maintaining
// partial-match state across iterations instead.
type JunctionSequence struct {
	Pattern []byte
	Kind    JunctionKind
}

// QuotingContext describes a delimiter that toggles quoting state,
// suppressing junction detection for bytes inside quotes.
type QuotingContext struct {
	Delimiter    byte // the quote character (e.g. '"')
	EscapePrefix byte // escape character inside quotes (e.g. '\\'); 0 means none
}

// ScannerSpec is the grammar-derived configuration for junction scanning.
type ScannerSpec struct {
	Junctions []JunctionByte     // single-byte delimiters (LUT fast path)
	Sequences []JunctionSequence // multi-byte delimiters (lookahead match)
	Quoting   []QuotingContext
}

// JunctionHit records a structural delimiter found during scanning.
type JunctionHit struct {
	Pos   int32
	Depth int16
	Kind  JunctionKind
	Byte  byte  // first byte of the delimiter
	Len   int16 // byte length of the delimiter (1 for single-byte)
}

// HitCounts summarizes junction hits by byte and kind. Used for
// estimating per-type node counts for arena pre-sizing.
type HitCounts struct {
	ByByte [256]int // count of hits per junction byte
	Opens  int      // total Open hits
	Closes int      // total Close hits
	Seps   int      // total Separator hits
	Quotes int      // number of quoting transitions (enter/exit pairs)
}

// CountHits summarizes a hit stream into per-byte and per-kind totals.
func CountHits(hits []JunctionHit) HitCounts {
	var c HitCounts
	for _, h := range hits {
		c.ByByte[h.Byte]++
		switch h.Kind {
		case JunctionOpen:
			c.Opens++
		case JunctionClose:
			c.Closes++
		case JunctionSeparator:
			c.Seps++
		}
	}
	return c
}

// Partition represents a region of input bounded by matched open/close
// junctions, with child partitions for nested structure.
type Partition struct {
	Start    int32
	End      int32
	Depth    int16
	Children []Partition
}
