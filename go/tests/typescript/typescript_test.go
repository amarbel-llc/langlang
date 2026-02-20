package typescript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"

	"github.com/stretchr/testify/require"
)

const (
	grammarPath = "../../../grammars/typescript.peg"
	testdataDir = "../../../testdata/typescript"
)

func TestTypeScriptTestFiles(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, testdataDir, ".ts")
}

func TestTypeScriptSnippets(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)
	tests := []struct {
		name  string
		input string
	}{
		{"let with type", "let x: number = 5;\n"},
		{"const with type", "const name: string = \"hello\";\n"},
		{"function with types", "function add(a: number, b: number): number { return a + b; }\n"},
		{"optional param", "function f(x?: string): void {}\n"},
		{"rest param typed", "function f(...args: string[]): void {}\n"},
		{"arrow with type", "const f = (x: number): number => x * 2;\n"},
		{"generic function", "function id<T>(x: T): T { return x; }\n"},
		{"interface", "interface Foo { x: number; y: string; }\n"},
		{"type alias", "type ID = string | number;\n"},
		{"type alias object", "type Point = { x: number; y: number };\n"},
		{"enum", "enum Color { Red, Green, Blue }\n"},
		{"const enum", "const enum Dir { Up, Down, Left, Right }\n"},
		{"string enum", "enum Status { Active = \"ACTIVE\", Inactive = \"INACTIVE\" }\n"},
		{"class with types", "class Foo { x: number; constructor(x: number) { this.x = x; } }\n"},
		{"abstract class", "abstract class Shape { abstract area(): number; }\n"},
		{"as expression", "let s = value as string;\n"},
		{"type assertion angle", "let n = <number>x;\n"},
		{"import type", "import type { Foo } from \"bar\";\n"},
		{"export interface", "export interface Config { debug: boolean; }\n"},
		{"export type", "export type ID = string;\n"},
		{"namespace", "namespace Util { export function f(): void {} }\n"},
		{"declare var", "declare var x: number;\n"},
		{"declare function", "declare function f(x: string): void;\n"},
		{"generic class", "class Box<T> { value: T; constructor(v: T) { this.value = v; } }\n"},
		{"union type", "let val: string | number = \"hello\";\n"},
		{"intersection type", "type AB = A & B;\n"},
		{"tuple type", "let t: [string, number] = [\"a\", 1];\n"},
		{"array type", "let arr: number[] = [1, 2, 3];\n"},
		{"non-null assertion", "let x = obj!.prop;\n"},
		{"keyof", "type Keys = keyof { a: 1; b: 2 };\n"},
		{"typeof in type", "type T = typeof someVar;\n"},
		{"conditional type", "type IsStr<T> = T extends string ? true : false;\n"},
		{"mapped type", "type RO<T> = { readonly [K in keyof T]: T[K] };\n"},
		{"satisfies", "const cfg = {} satisfies Config;\n"},
		{"implements", "class Foo implements Bar { x: number = 0; }\n"},
		{"extends generic", "interface Foo<T extends string> { val: T; }\n"},
		{"default type param", "type Container<T = string> = { value: T };\n"},
		{"class access modifiers", "class C { public a: number = 0; private b: string = \"\"; protected c: boolean = true; }\n"},
		{"definite assignment", "let x!: number;\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corpus.AssertParsesAll(t, matcher, []byte(tt.input), tt.name)
		})
	}
}

func BenchmarkParser(b *testing.B) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	if err != nil {
		b.Fatalf("failed to create matcher: %v", err)
	}

	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		b.Fatalf("failed to read testdata dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".ts" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(testdataDir, entry.Name()))
		if err != nil {
			b.Fatalf("failed to read %s: %v", entry.Name(), err)
		}
		b.Run(entry.Name(), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				_, _, err := matcher.Match(data)
				if err != nil {
					b.Fatalf("match error on %s: %v", entry.Name(), err)
				}
			}
		})
	}
}
