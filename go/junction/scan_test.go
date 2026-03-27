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
		{Pos: 0, Depth: 0, Kind: JunctionOpen, Byte: '{'},
		{Pos: 4, Depth: 1, Kind: JunctionSeparator, Byte: ':'},
		{Pos: 6, Depth: 1, Kind: JunctionSeparator, Byte: ','},
		{Pos: 10, Depth: 1, Kind: JunctionSeparator, Byte: ':'},
		{Pos: 11, Depth: 1, Kind: JunctionOpen, Byte: '['},
		{Pos: 13, Depth: 2, Kind: JunctionSeparator, Byte: ','},
		{Pos: 15, Depth: 1, Kind: JunctionClose, Byte: ']'},
		{Pos: 16, Depth: 0, Kind: JunctionClose, Byte: '}'},
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
		{Pos: 0, Depth: 0, Kind: JunctionOpen, Byte: '{'},
		{Pos: 4, Depth: 1, Kind: JunctionSeparator, Byte: ':'},
		{Pos: 11, Depth: 0, Kind: JunctionClose, Byte: '}'},
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
		{Pos: 0, Depth: 0, Kind: JunctionOpen, Byte: '{'},
		{Pos: 4, Depth: 1, Kind: JunctionSeparator, Byte: ':'},
		{Pos: 10, Depth: 0, Kind: JunctionClose, Byte: '}'},
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
