package main

import (
	"flag"
	"fmt"
	"os"

	langlang "github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/binary"
)

func runCodegen(args []string) {
	fs := flag.NewFlagSet("codegen", flag.ExitOnError)
	grammar := fs.String("grammar", "", "Path to the grammar file (required)")
	lang := fs.String("lang", "", "Output language: go (required)")
	outputPath := fs.String("output-path", "", "Path to the output file (required)")
	goPackage := fs.String("go-package", "wire", "Go package name")
	fs.Parse(args)

	if *grammar == "" || *lang == "" || *outputPath == "" {
		fmt.Fprintf(os.Stderr, "error: -grammar, -lang, and -output-path are required\n")
		os.Exit(1)
	}

	if *lang != "go" {
		fatal("codegen: unsupported language %q (only 'go' is supported)", *lang)
	}

	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.add_builtins", false)
	cfg.SetBool("grammar.handle_spaces", false)
	cfg.SetBool("grammar.captures", false)

	loader := langlang.NewRelativeImportLoader()
	db := langlang.NewDatabase(cfg, loader)

	ast, err := langlang.QueryAST(db, *grammar)
	if err != nil {
		fatal("Failed to parse grammar: %s", err.Error())
	}

	output, err := binary.Generate(ast, binary.Options{
		PackageName: *goPackage,
		SourceFile:  *grammar,
	})
	if err != nil {
		fatal("codegen failed: %s", err.Error())
	}

	if err := os.WriteFile(*outputPath, []byte(output), 0o644); err != nil {
		fatal("Can't write output: %s", err.Error())
	}
}
