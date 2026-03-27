package junction

import (
	"path/filepath"
	"runtime"
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


func TestAnalyzeForJunctionsJSON(t *testing.T) {
	spec, err := AnalyzeForJunctions(jsonGrammarPath())
	if err != nil {
		t.Fatalf("AnalyzeForJunctions: %v", err)
	}

	// Verify junction bytes.
	wantJunctions := map[byte]JunctionKind{
		'{': JunctionOpen,
		'}': JunctionClose,
		'[': JunctionOpen,
		']': JunctionClose,
		',': JunctionSeparator,
		':': JunctionSeparator,
	}

	gotJunctions := make(map[byte]JunctionKind)
	for _, jb := range spec.Junctions {
		gotJunctions[jb.Byte] = jb.Kind
	}

	for b, wantKind := range wantJunctions {
		gotKind, ok := gotJunctions[b]
		if !ok {
			t.Errorf("missing junction byte %q", b)
			continue
		}
		if gotKind != wantKind {
			t.Errorf("junction %q: got kind %d, want %d", b, gotKind, wantKind)
		}
	}

	// Should not have extra unexpected junctions (allow but warn).
	for _, jb := range spec.Junctions {
		if _, expected := wantJunctions[jb.Byte]; !expected {
			t.Logf("unexpected junction byte %q (kind %d)", jb.Byte, jb.Kind)
		}
	}

	// Verify quoting context.
	if len(spec.Quoting) == 0 {
		t.Fatal("expected at least one quoting context")
	}

	foundQuote := false
	for _, qc := range spec.Quoting {
		if qc.Delimiter == '"' {
			foundQuote = true
			if qc.EscapePrefix != '\\' {
				t.Errorf("quote escape prefix = %q, want '\\'", qc.EscapePrefix)
			}
		}
	}
	if !foundQuote {
		t.Error("missing quoting context for '\"'")
	}
}

func TestAnalyzeForJunctionsTOML(t *testing.T) {
	spec, err := AnalyzeForJunctions(tomlGrammarPath())
	if err != nil {
		t.Fatalf("AnalyzeForJunctions: %v", err)
	}

	wantJunctions := map[byte]JunctionKind{
		'[': JunctionOpen,
		']': JunctionClose,
		'{': JunctionOpen,
		'}': JunctionClose,
		'=': JunctionSeparator,
		'.': JunctionSeparator,
		',': JunctionSeparator,
	}

	gotJunctions := make(map[byte]JunctionKind)
	for _, jb := range spec.Junctions {
		gotJunctions[jb.Byte] = jb.Kind
	}

	for b, wantKind := range wantJunctions {
		gotKind, ok := gotJunctions[b]
		if !ok {
			t.Errorf("missing junction byte %q", b)
			continue
		}
		if gotKind != wantKind {
			t.Errorf("junction %q: got kind %d, want %d", b, gotKind, wantKind)
		}
	}

	for _, jb := range spec.Junctions {
		if _, expected := wantJunctions[jb.Byte]; !expected {
			t.Errorf("unexpected junction byte %q (kind %d)", jb.Byte, jb.Kind)
		}
	}

	// Verify quoting: " with \ escape, no false positives from lex rules.
	if len(spec.Quoting) != 1 {
		t.Fatalf("expected 1 quoting context, got %d", len(spec.Quoting))
	}
	if spec.Quoting[0].Delimiter != '"' || spec.Quoting[0].EscapePrefix != '\\' {
		t.Errorf("quoting = {delim=%q escape=%q}, want {delim='\"' escape='\\\\'}",
			spec.Quoting[0].Delimiter, spec.Quoting[0].EscapePrefix)
	}
}

func TestAnalyzeForJunctionsXML(t *testing.T) {
	spec, err := AnalyzeForJunctions(xmlGrammarPath())
	if err != nil {
		t.Fatalf("AnalyzeForJunctions: %v", err)
	}

	t.Logf("junctions (%d):", len(spec.Junctions))
	for _, jb := range spec.Junctions {
		kind := "?"
		switch jb.Kind {
		case JunctionOpen:
			kind = "Open"
		case JunctionClose:
			kind = "Close"
		case JunctionSeparator:
			kind = "Separator"
		}
		t.Logf("  %q -> %s", jb.Byte, kind)
	}

	t.Logf("quoting (%d):", len(spec.Quoting))
	for _, qc := range spec.Quoting {
		t.Logf("  delim=%q escape=%q", qc.Delimiter, qc.EscapePrefix)
	}
}

func TestAnalyzeForJunctionsGo(t *testing.T) {
	spec, err := AnalyzeForJunctions(goGrammarPath())
	if err != nil {
		t.Fatalf("AnalyzeForJunctions: %v", err)
	}

	t.Logf("junctions (%d):", len(spec.Junctions))
	for _, jb := range spec.Junctions {
		kind := "?"
		switch jb.Kind {
		case JunctionOpen:
			kind = "Open"
		case JunctionClose:
			kind = "Close"
		case JunctionSeparator:
			kind = "Separator"
		}
		t.Logf("  %q -> %s", jb.Byte, kind)
	}

	t.Logf("quoting (%d):", len(spec.Quoting))
	for _, qc := range spec.Quoting {
		t.Logf("  delim=%q escape=%q", qc.Delimiter, qc.EscapePrefix)
	}
}
