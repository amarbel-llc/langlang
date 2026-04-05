package junction

import (
	"testing"
)

var jsonSpec = ScannerSpec{
	Junctions: []JunctionByte{
		{'{', JunctionOpen},
		{'[', JunctionOpen},
		{'}', JunctionClose},
		{']', JunctionClose},
		{',', JunctionSeparator},
		{':', JunctionSeparator},
	},
	Quoting: []QuotingContext{
		{Delimiter: '"', EscapePrefix: '\\'},
	},
}

func TestScanJunctionsBasic(t *testing.T) {
	input := []byte(`{"a":1,"b":[2,3]}`)
	hits := ScanJunctions(input, jsonSpec)

	expected := []JunctionHit{
		{Pos: 0, Depth: 0, Kind: JunctionOpen, Byte: '{', Len: 1},
		{Pos: 4, Depth: 1, Kind: JunctionSeparator, Byte: ':', Len: 1},
		{Pos: 6, Depth: 1, Kind: JunctionSeparator, Byte: ',', Len: 1},
		{Pos: 10, Depth: 1, Kind: JunctionSeparator, Byte: ':', Len: 1},
		{Pos: 11, Depth: 1, Kind: JunctionOpen, Byte: '[', Len: 1},
		{Pos: 13, Depth: 2, Kind: JunctionSeparator, Byte: ',', Len: 1},
		{Pos: 15, Depth: 1, Kind: JunctionClose, Byte: ']', Len: 1},
		{Pos: 16, Depth: 0, Kind: JunctionClose, Byte: '}', Len: 1},
	}

	if len(hits) != len(expected) {
		t.Fatalf("got %d hits, want %d", len(hits), len(expected))
	}
	for i, got := range hits {
		if got != expected[i] {
			t.Errorf("hit[%d] = %+v, want %+v", i, got, expected[i])
		}
	}
}

func TestScanJunctionsQuoting(t *testing.T) {
	// Braces inside strings should not be detected as junctions.
	input := []byte(`{"k":"v{x}"}`)
	hits := ScanJunctions(input, jsonSpec)

	expected := []JunctionHit{
		{Pos: 0, Depth: 0, Kind: JunctionOpen, Byte: '{', Len: 1},
		{Pos: 4, Depth: 1, Kind: JunctionSeparator, Byte: ':', Len: 1},
		{Pos: 11, Depth: 0, Kind: JunctionClose, Byte: '}', Len: 1},
	}

	if len(hits) != len(expected) {
		t.Fatalf("got %d hits, want %d\nhits: %+v", len(hits), len(expected), hits)
	}
	for i, got := range hits {
		if got != expected[i] {
			t.Errorf("hit[%d] = %+v, want %+v", i, got, expected[i])
		}
	}
}

func TestScanJunctionsEscape(t *testing.T) {
	// Escaped quote inside string should not end the string.
	input := []byte(`{"k":"v\""}`)
	hits := ScanJunctions(input, jsonSpec)

	expected := []JunctionHit{
		{Pos: 0, Depth: 0, Kind: JunctionOpen, Byte: '{', Len: 1},
		{Pos: 4, Depth: 1, Kind: JunctionSeparator, Byte: ':', Len: 1},
		{Pos: 10, Depth: 0, Kind: JunctionClose, Byte: '}', Len: 1},
	}

	if len(hits) != len(expected) {
		t.Fatalf("got %d hits, want %d\nhits: %+v", len(hits), len(expected), hits)
	}
	for i, got := range hits {
		if got != expected[i] {
			t.Errorf("hit[%d] = %+v, want %+v", i, got, expected[i])
		}
	}
}

func TestScanJunctionsEmpty(t *testing.T) {
	hits := ScanJunctions([]byte{}, jsonSpec)
	if len(hits) != 0 {
		t.Errorf("expected no hits for empty input, got %d", len(hits))
	}
}

func TestScanJunctionsSequenceXML(t *testing.T) {
	// XML-like spec: < is Open, > is Close, </ is Close (overrides <).
	spec := ScannerSpec{
		Junctions: []JunctionByte{
			{'<', JunctionOpen},
			{'>', JunctionClose},
		},
		Sequences: []JunctionSequence{
			{Pattern: []byte("</"), Kind: JunctionClose},
		},
	}

	// <a><b></b></a>
	input := []byte(`<a><b></b></a>`)
	hits := ScanJunctions(input, spec)

	expected := []JunctionHit{
		// <a>
		{Pos: 0, Depth: 0, Kind: JunctionOpen, Byte: '<', Len: 1},  // <  depth 0→1
		{Pos: 2, Depth: 0, Kind: JunctionClose, Byte: '>', Len: 1}, // >  depth 1→0
		// <b>
		{Pos: 3, Depth: 0, Kind: JunctionOpen, Byte: '<', Len: 1},  // <  depth 0→1
		{Pos: 5, Depth: 0, Kind: JunctionClose, Byte: '>', Len: 1}, // >  depth 1→0
		// </b> — </ matches sequence (Close), not single-byte <
		{Pos: 6, Depth: -1, Kind: JunctionClose, Byte: '<', Len: 2}, // </  depth 0→-1
		{Pos: 9, Depth: -2, Kind: JunctionClose, Byte: '>', Len: 1}, // >   depth -1→-2
		// </a>
		{Pos: 10, Depth: -3, Kind: JunctionClose, Byte: '<', Len: 2}, // </  depth -2→-3
		{Pos: 13, Depth: -4, Kind: JunctionClose, Byte: '>', Len: 1}, // >   depth -3→-4
	}

	if len(hits) != len(expected) {
		t.Fatalf("got %d hits, want %d\nhits: %+v", len(hits), len(expected), hits)
	}
	for i, got := range hits {
		if got != expected[i] {
			t.Errorf("hit[%d] = %+v, want %+v", i, got, expected[i])
		}
	}
}

func TestScanJunctionsSequenceTOML(t *testing.T) {
	// TOML-like: [ is Open, ] is Close, [[ is Open (array-of-tables), ]] is Close.
	spec := ScannerSpec{
		Junctions: []JunctionByte{
			{'[', JunctionOpen},
			{']', JunctionClose},
			{'=', JunctionSeparator},
		},
		Sequences: []JunctionSequence{
			{Pattern: []byte("[["), Kind: JunctionOpen},
			{Pattern: []byte("]]"), Kind: JunctionClose},
		},
	}

	// [[items]]\nname = "a"\n[meta]\nk = "v"
	input := []byte("[[items]]\nname = \"a\"\n[meta]\nk = \"v\"")
	hits := ScanJunctions(input, spec)

	// [[ at pos 0 should match as a sequence (Open, Len=2)
	if len(hits) < 1 {
		t.Fatalf("got %d hits, want >= 1", len(hits))
	}
	first := hits[0]
	if first.Pos != 0 || first.Kind != JunctionOpen || first.Len != 2 {
		t.Errorf("first hit = %+v, want Pos=0 Kind=Open Len=2", first)
	}

	// ]] at pos 7 should match as a sequence (Close, Len=2)
	if len(hits) < 2 {
		t.Fatalf("got %d hits, want >= 2", len(hits))
	}
	second := hits[1]
	if second.Pos != 7 || second.Kind != JunctionClose || second.Len != 2 {
		t.Errorf("second hit = %+v, want Pos=7 Kind=Close Len=2", second)
	}

	// [ at pos 21 should match as single-byte Open (not [[)
	var metaHit *JunctionHit
	for i := range hits {
		if hits[i].Pos == 21 {
			metaHit = &hits[i]
			break
		}
	}
	if metaHit == nil {
		t.Fatal("no hit at pos 21 for [meta]")
	}
	if metaHit.Kind != JunctionOpen || metaHit.Len != 1 {
		t.Errorf("[meta] hit = %+v, want Kind=Open Len=1", *metaHit)
	}
}

func TestScanJunctionsSequenceLongestMatch(t *testing.T) {
	// Verify longest match wins: <!-- (4 bytes) should beat <! (2 bytes) and < (1 byte).
	spec := ScannerSpec{
		Junctions: []JunctionByte{
			{'<', JunctionOpen},
		},
		Sequences: []JunctionSequence{
			{Pattern: []byte("<!"), Kind: JunctionSeparator},
			{Pattern: []byte("<!--"), Kind: JunctionClose},
		},
	}

	input := []byte("<!-- comment -->")
	hits := ScanJunctions(input, spec)

	if len(hits) < 1 {
		t.Fatalf("got %d hits, want >= 1", len(hits))
	}
	if hits[0].Len != 4 || hits[0].Kind != JunctionClose {
		t.Errorf("hit[0] = %+v, want Len=4 Kind=Close (<!-- wins over <!)", hits[0])
	}
}

func TestScanJunctionsSequencePartialAtEnd(t *testing.T) {
	// Sequence trigger byte at end of input — not enough bytes to match.
	// Should fall through to single-byte.
	spec := ScannerSpec{
		Junctions: []JunctionByte{
			{'<', JunctionOpen},
		},
		Sequences: []JunctionSequence{
			{Pattern: []byte("</"), Kind: JunctionClose},
		},
	}

	input := []byte("abc<")
	hits := ScanJunctions(input, spec)

	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1", len(hits))
	}
	// < at end can't match </ (no room), falls through to single-byte Open.
	if hits[0].Kind != JunctionOpen || hits[0].Len != 1 {
		t.Errorf("hit = %+v, want Kind=Open Len=1 (single-byte fallback)", hits[0])
	}
}
