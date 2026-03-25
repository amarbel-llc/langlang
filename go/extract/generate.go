package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	pkg := detectPackageName(sourceFile)

	output, err := RenderFile(pkg, grammarPath, structs, rules)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(sourceFile), ".go")
	outPath := filepath.Join(filepath.Dir(sourceFile), base+"_extract.go")
	if err := os.WriteFile(outPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}

func detectPackageName(sourceFile string) string {
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return "main"
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "package "))
		}
	}
	return "main"
}
