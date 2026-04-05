package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clarete/langlang/go/junction"
)

// Generate is the main orchestrator. It reads a Go source file and grammar,
// cross-validates, and writes <source>_extract.go.
func Generate(sourceFile, grammarPath string) error {
	structs, err := Analyze(sourceFile)
	if err != nil {
		return fmt.Errorf("analyze structs: %w", err)
	}
	if len(structs) == 0 {
		return fmt.Errorf("no structs with ll: tags found in %s", sourceFile)
	}

	rules, err := AnalyzeGrammar(grammarPath)
	if err != nil {
		return fmt.Errorf("analyze grammar: %w", err)
	}

	structs, errs := Validate(structs, rules)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return fmt.Errorf("validation errors:\n  %s", strings.Join(msgs, "\n  "))
	}

	nameIDs := collectNameIDs(structs, rules)
	pkg := detectPackageName(sourceFile)

	output, err := RenderFile(pkg, grammarPath, nameIDs, structs, rules)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(sourceFile), ".go")
	outPath := filepath.Join(filepath.Dir(sourceFile), base+"_extract.go")
	if err := os.WriteFile(outPath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}

func collectNameIDs(structs []StructInfo, rules map[string]RuleInfo) []NameIDEntry {
	seen := map[string]bool{}
	var entries []NameIDEntry
	for _, si := range structs {
		for _, f := range si.Fields {
			if seen[f.LLTag] {
				continue
			}
			seen[f.LLTag] = true
			if rule, ok := rules[f.LLTag]; ok {
				entries = append(entries, NameIDEntry{Name: f.LLTag, ID: rule.NameID})
			}
		}
	}
	return entries
}

// GenerateJen is like Generate but uses the jennifer-based emitter.
func GenerateJen(sourceFile, grammarPath string) error {
	structs, err := Analyze(sourceFile)
	if err != nil {
		return fmt.Errorf("analyze structs: %w", err)
	}
	if len(structs) == 0 {
		return fmt.Errorf("no structs with ll: tags found in %s", sourceFile)
	}

	rules, err := AnalyzeGrammar(grammarPath)
	if err != nil {
		return fmt.Errorf("analyze grammar: %w", err)
	}

	structs, errs := Validate(structs, rules)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return fmt.Errorf("validation errors:\n  %s", strings.Join(msgs, "\n  "))
	}

	nameIDs := collectNameIDs(structs, rules)
	pkg := detectPackageName(sourceFile)

	output, err := RenderFileJen(pkg, grammarPath, nameIDs, structs, rules)
	if err != nil {
		return fmt.Errorf("render jen: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(sourceFile), ".go")
	outPath := filepath.Join(filepath.Dir(sourceFile), base+"_extract.go")
	if err := os.WriteFile(outPath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}

// GenerateViews produces zero-allocation view types from a grammar file.
// Unlike Generate, it does not require a Go source file with struct definitions.
// The output file is written as <basename>_views.go alongside the grammar.
func GenerateViews(grammarPath, pkg, outDir, rootRule string) error {
	rules, err := AnalyzeGrammar(grammarPath)
	if err != nil {
		return fmt.Errorf("analyze grammar: %w", err)
	}

	output, err := RenderViewsFile(pkg, grammarPath, rules, rootRule)
	if err != nil {
		return fmt.Errorf("render views: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(grammarPath), filepath.Ext(grammarPath))
	outPath := filepath.Join(outDir, base+"_views.go")
	if err := os.WriteFile(outPath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}

// GenerateViewsJen is like GenerateViews but uses the jennifer-based emitter.
func GenerateViewsJen(grammarPath, pkg, outDir, rootRule string) error {
	rules, err := AnalyzeGrammar(grammarPath)
	if err != nil {
		return fmt.Errorf("analyze grammar: %w", err)
	}

	output, err := RenderViewsFileJen(pkg, grammarPath, rules, rootRule)
	if err != nil {
		return fmt.Errorf("render views jen: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(grammarPath), filepath.Ext(grammarPath))
	outPath := filepath.Join(outDir, base+"_views.go")
	if err := os.WriteFile(outPath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}

// GenerateArena produces arena types and extraction functions alongside
// the existing heap-based extraction. Output: <source>_arena.go.
func GenerateArena(sourceFile, grammarPath string, includeScan bool) error {
	structs, err := Analyze(sourceFile)
	if err != nil {
		return fmt.Errorf("analyze structs: %w", err)
	}
	if len(structs) == 0 {
		return fmt.Errorf("no structs with ll: tags found in %s", sourceFile)
	}

	rules, err := AnalyzeGrammar(grammarPath)
	if err != nil {
		return fmt.Errorf("analyze grammar: %w", err)
	}

	structs, errs := Validate(structs, rules)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return fmt.Errorf("validation errors:\n  %s", strings.Join(msgs, "\n  "))
	}

	var spec *junction.ScannerSpec
	if includeScan {
		s, err := junction.AnalyzeForJunctions(grammarPath)
		if err != nil {
			return fmt.Errorf("analyze junctions: %w", err)
		}
		spec = &s
	}

	nameIDs := collectNameIDs(structs, rules)
	pkg := detectPackageName(sourceFile)

	output, err := RenderArenaFileJen(pkg, grammarPath, nameIDs, structs, rules, spec)
	if err != nil {
		return fmt.Errorf("render arena: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(sourceFile), ".go")
	outPath := filepath.Join(filepath.Dir(sourceFile), base+"_arena.go")
	if err := os.WriteFile(outPath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}

func detectPackageName(sourceFile string) string {
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return "main"
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "package "); ok {
			return strings.TrimSpace(after)
		}
	}
	return "main"
}
