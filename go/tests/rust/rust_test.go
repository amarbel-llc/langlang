package rust

import (
	"testing"

	"github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	grammarPath     = "../../../grammars/rust.peg"     // Edition 2024 (gen reserved, gen blocks)
	grammarPath2021 = "../../../grammars/rust2021.peg" // Edition 2021 (gen is valid identifier)
	testdataDir     = "../../../testdata/rust"
)

func TestRustTestFiles(t *testing.T) {
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, testdataDir, ".rs")
}

var rustSnippets = []struct {
	name string
	src  string
}{
	// --- basics ---
	{"empty-fn", "fn main() {}\n"},
	{"let-binding", "fn main() { let x: i32 = 42; }\n"},
	{"let-mut", "fn main() { let mut v = vec![1, 2, 3]; v.push(4); }\n"},
	{"struct-def", "struct Point { x: f64, y: f64 }\n"},
	{"tuple-struct", "struct Color(u8, u8, u8);\n"},
	{"unit-struct", "struct Unit;\n"},
	{"enum-simple", "enum Dir { N, S, E, W }\n"},
	{"enum-data", "enum Opt<T> { Some(T), None }\n"},
	{"trait-def", "trait Foo { fn bar(&self) -> i32; }\n"},
	{"impl-block", "struct S;\nimpl S { fn new() -> Self { S } }\n"},
	{"use-star", "use std::collections::*;\n"},
	{"use-group", "use std::io::{self, Read};\n"},
	{"extern-crate", "extern crate alloc;\n"},
	{"type-alias", "type Result<T> = std::result::Result<T, Error>;\n"},
	{"const-item", "const MAX: usize = 100;\n"},
	{"static-item", "static COUNT: i32 = 0;\n"},
	{"if-else", "fn f() { if true { 1 } else { 2 }; }\n"},
	{"match-expr", "fn f(x: i32) { match x { 0 => 0, _ => 1, }; }\n"},
	{"loop-break", "fn f() { loop { break; } }\n"},
	{"while-loop", "fn f() { while true { break; } }\n"},
	{"for-loop", "fn f() { for i in 0..10 { drop(i); } }\n"},
	{"closure-basic", "fn f() { let c = |x: i32| x + 1; }\n"},
	{"async-fn", "async fn fetch() -> u32 { 42 }\n"},
	{"question-mark", "fn f() -> Result<(), E> { foo()?; Ok(()) }\n"},
	{"attribute", "#[derive(Debug)]\nstruct S;\n"},
	{"inner-attr", "#![allow(unused)]\nfn main() {}\n"},
	{"lifetime-fn", "fn f<'a>(x: &'a str) -> &'a str { x }\n"},
	{"where-clause", "fn f<T>(x: T) where T: Clone + Send { drop(x); }\n"},
	{"unsafe-block", "fn f() { unsafe { drop(0); } }\n"},
	{"macro-println", "fn main() { println!(\"hello {}\", 42); }\n"},
	{"macro-vec", "fn main() { let v = vec![1, 2, 3]; }\n"},

	// --- nested block comments ---
	{"nested-comment", "/* outer /* inner */ end */ fn main() {}\n"},
	{"deeply-nested-comment", "/* a /* b /* c */ d */ e */ fn main() {}\n"},

	// --- float literals in patterns ---
	{"float-range-pattern", "fn f(x: f64) { match x { 0.0..=1.0 => {} _ => {} } }\n"},
	{"neg-float-range-pattern", "fn f(x: f64) { match x { -1.0..=1.0 => {} _ => {} } }\n"},

	// --- half-open range patterns ---
	{"half-open-range-excl-upper", "fn f(x: i32) { match x { ..5 => {} _ => {} } }\n"},
	{"half-open-range-incl-upper", "fn f(x: i32) { match x { ..=5 => {} _ => {} } }\n"},
	{"half-open-range-lower", "fn f(x: i32) { match x { 5.. => {} _ => {} } }\n"},

	// --- raw address-of ---
	{"raw-const-ptr", "fn f(x: i32) { let p = &raw const x; }\n"},
	{"raw-mut-ptr", "fn f(mut x: i32) { let p = &raw mut x; }\n"},

	// --- complex generics ---
	{"nested-generics", "fn f() -> Vec<Option<i32>> { vec![] }\n"},
	{"deeply-nested-generics", "fn f() -> Vec<Vec<Vec<i32>>> { vec![] }\n"},
	{"complex-trait-object", "fn f() -> Box<dyn Fn(&str) -> Result<(), Box<dyn std::error::Error>>> { todo!() }\n"},
	{"trait-object-send-static", "fn f() -> Box<dyn Fn() + Send + 'static> { todo!() }\n"},

	// --- const generics ---
	{"const-generic-struct", "struct Arr<const N: usize> { data: [u8; N] }\n"},
	{"const-generic-impl", "impl<const N: usize> Arr<N> { fn len(&self) -> usize { N } }\n"},
	{"const-generic-expr", "fn f() { let a: Arr<{ 1 + 2 }> = todo!(); }\n"},

	// --- higher-ranked trait bounds ---
	{"hrtb-where", "fn f<F>(func: F) where F: for<'a> Fn(&'a str) -> &'a str { drop(func); }\n"},
	{"hrtb-bound", "fn f<F: for<'a> Fn(&'a str)>(func: F) { drop(func); }\n"},

	// --- qualified paths ---
	{"qualified-path-type", "fn f() { let x = <Vec<i32> as IntoIterator>::into_iter(vec![]); }\n"},
	{"qualified-path-assoc", "fn f() { let _: <Vec<i32> as IntoIterator>::Item; }\n"},

	// --- let-else ---
	{"let-else", "fn f(x: Option<i32>) { let Some(v) = x else { return; }; drop(v); }\n"},

	// --- if-let chains ---
	{"if-let-chain", "fn f(x: Option<i32>, y: Option<i32>) { if let Some(a) = x && let Some(b) = y { drop(a + b); } }\n"},

	// --- complex match ---
	{"match-ref-mut-guard", "fn f(x: Option<i32>) { match x { Some(ref mut y) if *y > 0 => { *y -= 1; } _ => {} } }\n"},
	{"match-or-pattern", "fn f(x: i32) { match x { 1 | 2 | 3 => {} _ => {} } }\n"},
	{"match-struct-pattern", "struct P { x: i32 } fn f(p: P) { match p { P { x: 0 } => {} P { x, .. } => { drop(x); } } }\n"},
	{"match-tuple-struct-pat", "enum E { A(i32, i32) } fn f(e: E) { match e { E::A(x, y) => { drop(x + y); } } }\n"},
	{"match-binding", "fn f(x: Option<i32>) { match x { y @ Some(1..=5) => { drop(y); } _ => {} } }\n"},

	// --- closures ---
	{"closure-move", "fn f() { let s = String::new(); let c = move || drop(s); c(); }\n"},
	{"closure-return-type", "fn f() { let c = |x: i32| -> i32 { x + 1 }; }\n"},
	{"closure-no-params", "fn f() { let c = || 42; }\n"},
	{"async-closure", "fn f() { let c = async || 42; }\n"},
	{"async-move-closure", "fn f() { let c = async move || 42; }\n"},

	// --- complex types ---
	{"fn-ptr-type", "type F = fn(i32, i32) -> bool;\n"},
	{"fn-ptr-unsafe", "type F = unsafe extern \"C\" fn(*const u8) -> i32;\n"},
	{"impl-trait-return", "fn f() -> impl Iterator<Item = i32> { vec![1].into_iter() }\n"},
	{"impl-trait-multi-bound", "fn f() -> impl Clone + Send + 'static { 42 }\n"},
	{"dyn-trait-multi-bound", "fn f() -> Box<dyn Clone + Send + 'static> { todo!() }\n"},
	{"raw-pointer-types", "fn f(p: *const u8, q: *mut u8) { drop(p); drop(q); }\n"},
	{"never-type", "fn f() -> ! { loop {} }\n"},
	{"inferred-type", "fn f() { let x: Vec<_> = vec![1, 2, 3]; }\n"},
	{"array-type", "fn f() { let a: [u8; 4] = [0; 4]; }\n"},
	{"slice-ref-type", "fn f(s: &[u8]) { drop(s); }\n"},
	{"double-ref", "fn f(x: &&i32) { drop(x); }\n"},
	{"bare-fn-type-hrtb", "type F = for<'a> fn(&'a str) -> &'a str;\n"},

	// --- associated types ---
	{"assoc-type-bound", "trait I { type Item; } fn f<T: I<Item = i32>>() {}\n"},
	{"assoc-type-in-where", "fn f<T>() where T: Iterator, T::Item: Clone {}\n"},

	// --- macro_rules ---
	{"macro-rules-simple", "macro_rules! my_mac { ($e:expr) => { $e + 1 }; }\n"},
	{"macro-rules-multi-arm", "macro_rules! m { () => {}; ($x:tt) => { $x }; ($($x:tt)*) => {}; }\n"},

	// --- complex attributes ---
	{"cfg-attr", "#[cfg(target_os = \"linux\")]\nfn linux_only() {}\n"},
	{"cfg-attr-nested", "#[cfg_attr(feature = \"serde\", derive(Serialize, Deserialize))]\nstruct S;\n"},
	{"doc-attr-macro", "#[doc = \"hello\"]\nfn f() {}\n"},
	{"multi-derive", "#[derive(Debug, Clone, PartialEq, Eq, Hash)]\nstruct S;\n"},

	// --- async/await ---
	{"await-chain", "async fn f() { foo().await.bar().await; }\n"},
	{"async-block-move", "fn f() { let fut = async move { 42 }; }\n"},

	// --- complex expressions ---
	{"chained-methods", "fn f() { vec![1, 2, 3].iter().map(|x| x + 1).filter(|x| *x > 2).collect::<Vec<_>>(); }\n"},
	{"turbofish", "fn f() { let x = \"42\".parse::<i32>(); }\n"},
	{"struct-update", "struct S { a: i32, b: i32 } fn f() { let s = S { a: 1, ..S { a: 0, b: 0 } }; }\n"},
	{"labeled-block", "fn f() { let x = 'blk: { break 'blk 42; }; }\n"},
	{"labeled-loop-break-val", "fn f() { let x = 'outer: loop { break 'outer 42; }; }\n"},
	{"nested-if-let", "fn f(x: Option<Option<i32>>) { if let Some(Some(v)) = x { drop(v); } }\n"},
	{"cast-chain", "fn f() { let x = 1u8 as u16 as u32 as u64; }\n"},
	{"range-expressions", "fn f() { let _ = 0..10; let _ = 0..=10; let _ = ..10; let _ = 10..; let _ = ..; }\n"},

	// --- union ---
	{"union-def", "union U { a: i32, b: f32 }\n"},
	{"union-access", "union U { a: i32, b: f32 } fn f(u: U) { unsafe { drop(u.a); } }\n"},

	// --- extern blocks ---
	{"extern-c-block", "extern \"C\" { fn printf(fmt: *const u8, ...) -> i32; }\n"},
	{"extern-fn", "extern \"C\" fn handler(sig: i32) { drop(sig); }\n"},

	// --- raw strings ---
	{"raw-string", "fn f() { let s = r\"hello\"; }\n"},
	{"raw-string-hashes", "fn f() { let s = r#\"contains \"quotes\"\"#; }\n"},
	{"raw-string-multi-hash", "fn f() { let s = r##\"contains \"# inside\"##; }\n"},
	{"byte-string-raw", "fn f() { let b = br\"bytes\"; }\n"},

	// --- numeric literals ---
	{"hex-lit", "fn f() { let x = 0xFF_u8; }\n"},
	{"octal-lit", "fn f() { let x = 0o77; }\n"},
	{"binary-lit", "fn f() { let x = 0b1010; }\n"},
	{"float-suffix", "fn f() { let x = 1.0f64; }\n"},
	{"underscore-in-nums", "fn f() { let x = 1_000_000; let y = 1_000.000_1; }\n"},

	// --- negative impl ---
	{"negative-impl", "impl !Send for S {}\n"},

	// --- where clause forms ---
	{"where-lifetime", "fn f<'a, 'b>(x: &'a str) where 'a: 'b { drop(x); }\n"},
	{"where-hrtb", "fn f<T>() where for<'a> &'a T: IntoIterator {}\n"},

	// --- tuple indexing ---
	{"tuple-index", "fn f() { let t = (1, 2); let x = t.0 + t.1; }\n"},
	{"nested-tuple-index", "fn f() { let t = ((1, 2), 3); let x = t.0.1; }\n"},

	// --- static closures ---
	{"static-closure", "fn f() { let c = static || 42; }\n"},

	// --- or-patterns ---
	{"or-pattern-let", "fn f(r: Result<i32, i32>) { let (Ok(x) | Err(x)) = r; drop(x); }\n"},

	// --- const block ---
	{"const-block-expr", "fn f() { let x = const { 1 + 2 }; }\n"},

	// --- method on literal ---
	{"method-on-int", "fn f() { 42i32.to_string(); }\n"},
	{"method-on-float", "fn f() { 1.0f64.sin(); }\n"},

	// --- macro invocation in pattern position ---
	{"macro-in-pattern", "macro_rules! T { ($x:tt) => { $x }; }\nfn f(x: i32) { match x { T![0] => {} _ => {} } }\n"},
	{"macro-in-match-or", "macro_rules! T { ($x:tt) => { $x }; }\nfn f(x: i32) { match x { T![0] | T![1] => {} _ => {} } }\n"},

	// --- GAT where clause (after = Type) ---
	{"gat-where-clause", "trait Foo { type Bar<'a> = () where Self: 'a; }\n"},
	{"gat-impl-where", "struct S; trait T { type O<'a>; } impl T for S { type O<'a> = &'a str where Self: 'a; }\n"},
	{"gat-multi-where", "trait T { type F<'a, 'b> = () where 'a: 'b, Self: 'a; }\n"},

	// --- use<Self> precise capturing ---
	{"use-bound-self", "trait T { fn f(&self) -> impl Sized + use<Self>; }\n"},

	// --- extern type declarations ---
	{"extern-type", "extern \"C\" { type Opaque; }\n"},
	{"extern-type-and-fn", "extern \"C\" { type Global; fn get_global() -> *const Global; }\n"},

	// --- Edition 2024: gen keyword and gen blocks (RFC 3513) ---
	{"edition2024-gen-keyword-reserved", "fn r#gen() { }\n"},
	{"edition2024-gen-block", "fn f() { let _ = gen { }; }\n"},
	{"edition2024-gen-block-yield", "fn f() { let _ = gen { yield 1; }; }\n"},
	{"edition2024-gen-move-block", "fn f() { let _ = gen move { yield (); }; }\n"},

	// --- closure lifetime binders (stable since 1.83) ---
	{"closure-for-lifetime", "fn f() { let _: fn(&str) -> &str = for<'a> |x: &'a str| -> &'a str { x }; }\n"},
	{"closure-for-lifetime-multi", "fn f() { let _ = for<'a, 'b> |_x: &'a u8, _y: &'b u8| {}; }\n"},
	{"closure-for-async-move", "fn f() { let _ = for<'a> async move |_x: &'a u8| {}; }\n"},

	// --- if-let match guards (let_chains, stable since 1.87) ---
	{"match-if-let-guard", "fn f(x: Option<i32>) { match x { Some(v) if let 0..=10 = v => {} _ => {} } }\n"},
	{"match-if-let-guard-chain", "fn f(x: Option<Option<i32>>) { match x { Some(v) if let Some(n) = v && n > 0 => {} _ => {} } }\n"},
	{"match-if-let-guard-expr-then-let", "fn f(x: i32, y: Option<i32>) { match x { n if n > 0 && let Some(v) = y => {} _ => {} } }\n"},
}

func TestRustSnippets(t *testing.T) {
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
	require.NoError(t, err)
	for _, tc := range rustSnippets {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.src)
			tree, n, err := matcher.Match(data)
			if !assert.NoError(t, err, "failed to parse snippet %s", tc.name) {
				if perr, ok := err.(langlang.ParsingError); ok {
					loc := tree.Location(perr.End)
					t.Logf("  error at %d:%d: %s", loc.Line, loc.Column, perr.Message)
				}
				t.Logf("  input: %q", tc.src)
				return
			}
			assert.Equal(t, len(data), n,
				"parser did not consume all input for snippet %s (consumed %d of %d)",
				tc.name, n, len(data))
		})
	}
}

func TestRustSingleSnippet(t *testing.T) {
	// Test multiple variants to isolate the async move issue
	cases := []struct{ name, src string }{
		{"async-stmt", "fn f() { async move {}; }\n"},
		{"async-let", "fn f() { let x = async move {}; }\n"},
		{"async-no-move-let", "fn f() { let x = async {}; }\n"},
		{"unsafe-let", "fn f() { let x = unsafe { 1 }; }\n"},
		{"block-let", "fn f() { let x = { 1 }; }\n"},
		{"async-call", "fn f() { foo(async move {}); }\n"},
		{"async-move-plus", "fn f() { let x = async move { 1 } ; }\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := langlang.NewConfig()
			cfg.SetBool("grammar.handle_spaces", false)
			matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
			require.NoError(t, err)
			data := []byte(tc.src)
			tree, n, err := matcher.Match(data)
			if err != nil {
				if perr, ok := err.(langlang.ParsingError); ok {
					loc := tree.Location(perr.End)
					t.Errorf("error at %d:%d: %s", loc.Line, loc.Column, perr.Message)
				} else {
					t.Errorf("match error: %v", err)
				}
				return
			}
			if n != len(data) {
				t.Errorf("consumed %d of %d bytes", n, len(data))
			}
		})
	}
}

// TestRustEdition2021Grammar verifies rust2021.peg parses Edition
// 2021 code where `gen` is a valid identifier (e.g. fn gen() {}). Use
// rust2021.peg for corpora or crates that have not migrated to
// Edition 2024.
func TestRustEdition2021Grammar(t *testing.T) {
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath2021, cfg)
	require.NoError(t, err)
	for _, src := range []string{
		"fn gen() { }\n",
		"fn main() { gen(); }\n",
		"fn f() { let gen = 1; }\n",
	} {
		data := []byte(src)
		tree, n, err := matcher.Match(data)
		require.NoError(t, err, "Edition 2021 grammar should parse %q", src)
		assert.Equal(t, len(data), n, "should consume all input for %q", src)
		_ = tree
	}
}
