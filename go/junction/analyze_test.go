package junction

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func jsonGrammarPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// junction/ is inside go/, docs/ is at repo root (go/..)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "live", "assets", "examples", "json", "json.peg")
}

func tomlGrammarPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "examples", "toml-extract", "toml.peg")
}

func xmlGrammarPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "live", "assets", "examples", "xml", "xml.peg")
}

func goGrammarPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "grammars", "go.peg")
}

type analyzerTestCase struct {
	name        string
	grammarPath string
	junctions   map[byte]JunctionKind
	quoting     []QuotingContext
}

func TestAnalyzeForJunctions(t *testing.T) {
	tests := []analyzerTestCase{
		{
			name:        "JSON",
			grammarPath: jsonGrammarPath(),
			junctions: map[byte]JunctionKind{
				'{': JunctionOpen,
				'}': JunctionClose,
				'[': JunctionOpen,
				']': JunctionClose,
				',': JunctionSeparator,
				':': JunctionSeparator,
			},
			quoting: []QuotingContext{
				{Delimiter: '"', EscapePrefix: '\\'},
			},
		},
		{
			name:        "TOML",
			grammarPath: tomlGrammarPath(),
			junctions: map[byte]JunctionKind{
				'[': JunctionOpen,
				']': JunctionClose,
				'{': JunctionOpen,
				'}': JunctionClose,
				'=': JunctionSeparator,
				'.': JunctionSeparator,
				',': JunctionSeparator,
			},
			quoting: []QuotingContext{
				{Delimiter: '"', EscapePrefix: '\\'},
			},
		},
		{
			// XML has limited single-byte junction coverage due to
			// multi-byte delimiters (</>, <?, etc). Pin current output
			// as regression baseline.
			name:        "XML",
			grammarPath: xmlGrammarPath(),
			junctions: map[byte]JunctionKind{
				'<': JunctionOpen,
				'>': JunctionClose,
				'=': JunctionOpen,
				'"': JunctionClose,
			},
			quoting: []QuotingContext{
				{Delimiter: '"', EscapePrefix: 0},
			},
		},
		{
			// Go grammar uses indirect delimiter references (LBRACE <- '{' Skip).
			// Currently {/} are classified as Separator because the
			// analyzer doesn't fully resolve indirection in bracket
			// detection. Pin current output as regression baseline.
			name:        "Go",
			grammarPath: goGrammarPath(),
			junctions: map[byte]JunctionKind{
				'`':  JunctionOpen,
				'"':  JunctionOpen,
				',':  JunctionSeparator,
				'{':  JunctionSeparator,
				'}':  JunctionSeparator,
				'(':  JunctionSeparator,
				')':  JunctionSeparator,
				'|':  JunctionSeparator,
				';':  JunctionSeparator,
				':':  JunctionSeparator,
			},
			quoting: []QuotingContext{
				{Delimiter: '`', EscapePrefix: 0},
				{Delimiter: '"', EscapePrefix: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := AnalyzeForJunctions(tt.grammarPath)
			if err != nil {
				t.Fatalf("AnalyzeForJunctions: %v", err)
			}

			assertJunctions(t, spec.Junctions, tt.junctions)
			assertQuoting(t, spec.Quoting, tt.quoting)
		})
	}
}

func assertJunctions(t *testing.T, got []JunctionByte, want map[byte]JunctionKind) {
	t.Helper()

	gotMap := make(map[byte]JunctionKind)
	for _, jb := range got {
		gotMap[jb.Byte] = jb.Kind
	}

	for b, wantKind := range want {
		gotKind, ok := gotMap[b]
		if !ok {
			t.Errorf("missing junction byte %s", fmtByte(b))
			continue
		}
		if gotKind != wantKind {
			t.Errorf("junction %s: got %s, want %s",
				fmtByte(b), fmtKind(gotKind), fmtKind(wantKind))
		}
	}

	for _, jb := range got {
		if _, expected := want[jb.Byte]; !expected {
			t.Errorf("unexpected junction byte %s (%s)", fmtByte(jb.Byte), fmtKind(jb.Kind))
		}
	}
}

func assertQuoting(t *testing.T, got []QuotingContext, want []QuotingContext) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("quoting count: got %d, want %d", len(got), len(want))
		for _, qc := range got {
			t.Logf("  got: delim=%s escape=%s", fmtByte(qc.Delimiter), fmtByte(qc.EscapePrefix))
		}
		return
	}

	// Sort both by delimiter for stable comparison.
	sortQuoting := func(s []QuotingContext) {
		sort.Slice(s, func(i, j int) bool {
			return s[i].Delimiter < s[j].Delimiter
		})
	}

	gotSorted := make([]QuotingContext, len(got))
	copy(gotSorted, got)
	sortQuoting(gotSorted)

	wantSorted := make([]QuotingContext, len(want))
	copy(wantSorted, want)
	sortQuoting(wantSorted)

	for i := range wantSorted {
		if gotSorted[i].Delimiter != wantSorted[i].Delimiter {
			t.Errorf("quoting[%d] delimiter: got %s, want %s",
				i, fmtByte(gotSorted[i].Delimiter), fmtByte(wantSorted[i].Delimiter))
		}
		if gotSorted[i].EscapePrefix != wantSorted[i].EscapePrefix {
			t.Errorf("quoting[%d] escape: got %s, want %s",
				i, fmtByte(gotSorted[i].EscapePrefix), fmtByte(wantSorted[i].EscapePrefix))
		}
	}
}

func fmtByte(b byte) string {
	if b == 0 {
		return "0x00"
	}
	return fmt.Sprintf("%q", rune(b))
}

func fmtKind(k JunctionKind) string {
	switch k {
	case JunctionOpen:
		return "Open"
	case JunctionClose:
		return "Close"
	case JunctionSeparator:
		return "Separator"
	default:
		return fmt.Sprintf("Kind(%d)", k)
	}
}
