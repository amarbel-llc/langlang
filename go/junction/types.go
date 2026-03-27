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

// QuotingContext describes a delimiter that toggles quoting state,
// suppressing junction detection for bytes inside quotes.
type QuotingContext struct {
	Delimiter    byte // the quote character (e.g. '"')
	EscapePrefix byte // escape character inside quotes (e.g. '\\'); 0 means none
}

// ScannerSpec is the grammar-derived configuration for junction scanning.
type ScannerSpec struct {
	Junctions []JunctionByte
	Quoting   []QuotingContext
}

// JunctionHit records a structural byte found during scanning.
type JunctionHit struct {
	Pos   int32
	Depth int16
	Kind  JunctionKind
	Byte  byte
}

// Partition represents a region of input bounded by matched open/close
// junctions, with child partitions for nested structure.
type Partition struct {
	Start    int32
	End      int32
	Depth    int16
	Children []Partition
}
