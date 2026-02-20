package regex

import (
	"fmt"
	"regexp/syntax"
	"testing"

	langlang "github.com/clarete/langlang/go"
	"github.com/stretchr/testify/require"
)

const grammarPath = "../../../grammars/regex.peg"

// regexCase holds one test pattern and the expected outcome for the
// langlang grammar.  Every pattern is first validated by regexp/syntax
// so we know it is a real regex.
type regexCase struct {
	pattern string
	accept  bool // true = grammar should accept; false = known unsupported
}

// Patterns grouped by the feature they exercise.  The `accept` field
// records whether the grammar in regex.peg is expected to handle it.
// When the grammar is extended, flip `accept` to true and re-run.

var literalCases = []regexCase{
	{"a", true},
	{"abc", true},
	{"hello", true},
	{"AbCdEf", true},
}

var dotCases = []regexCase{
	{".", true},
	{"..", true},
}

var quantifierCases = []regexCase{
	// greedy quantifiers
	{"a*", true},
	{"a+", true},
	{"a?", true},
	{"a{3}", true},
	{"a{3,5}", true},
	{"a{3,}", true},

	// lazy quantifiers
	{"a*?", true},
	{"a+?", true},
	{"a??", true},
	{"a{3}?", true},
	{"a{3,5}?", true},
	{"a{3,}?", true},
}

var alternationCases = []regexCase{
	{"a|b", true},
	{"abc|def", true},
	{"a|b|c", true},
}

var groupCases = []regexCase{
	{"(a)", true},
	{"(abc)", true},
	{"(a|b)", true},
	{"(a)(b)", true},
	{"((a))", true},
	{"(a*)", true},
	{"(a+b)*", true},
}

var charClassCases = []regexCase{
	{"[a-z]", true},
	{"[A-Z]", true},
	{"[^a-z]", true},
	{"[abc]", true},
	{"[a-zA-Z]", true},
	{"[^abc]", true},
}

var posixClassCases = []regexCase{
	{"[[:alnum:]]", true},
	{"[[:alpha:]]", true},
	{"[[:ascii:]]", true},
	{"[[:blank:]]", true},
	{"[[:cntrl:]]", true},
	{"[[:digit:]]", true},
	{"[[:graph:]]", true},
	{"[[:lower:]]", true},
	{"[[:print:]]", true},
	{"[[:punct:]]", true},
	{"[[:space:]]", true},
	{"[[:upper:]]", true},
	{"[[:word:]]", true},
	{"[[:xdigit:]]", true},
}

var perlEscapeCases = []regexCase{
	{`\d`, true},
	{`\D`, true},
	{`\s`, true},
	{`\S`, true},
	{`\w`, true},
	{`\W`, true},
}

var unicodePropCases = []regexCase{
	{`\p{L}`, true},
	{`\P{L}`, true},
	{`\p{N}`, true},
}

var anchorCases = []regexCase{
	{`^abc`, true},
	{`abc$`, true},
	{`\babc\b`, true},
}

var specialGroupCases = []regexCase{
	{`(?:abc)`, true},
	{`(?i)abc`, true},
	{`(?P<name>abc)`, true},
	{`(?i:abc)`, true},
	{`(?-s:.)`, true},
	{`(?-s:.+)`, true},
	{`(?im:foo)`, true},
}

var digitLiteralCases = []regexCase{
	{`1`, true},
	{`123`, true},
}

var escapedMetaCases = []regexCase{
	{`\+`, true},
	{`\.`, true},
	{`\\`, true},
}

var combinedCases = []regexCase{
	{"[a-z]+", true},
	{"[A-Za-z]+", true},
	{"(a|b)+", true},
	{"a.b", true},
	{".*", true},
	{".+", true},
	{`\d+`, true},
	{`\w+`, true},
	{`\d{3}`, true},
	{"[a-z]{2,4}", true},
	{"(ab)+", true},
}

func TestRegexGrammarDifferential(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	groups := []struct {
		name  string
		cases []regexCase
	}{
		{"Literal", literalCases},
		{"Dot", dotCases},
		{"Quantifier", quantifierCases},
		{"Alternation", alternationCases},
		{"Group", groupCases},
		{"CharClass", charClassCases},
		{"POSIXClass", posixClassCases},
		{"PerlEscape", perlEscapeCases},
		{"UnicodeProp", unicodePropCases},
		{"Anchor", anchorCases},
		{"SpecialGroup", specialGroupCases},
		{"DigitLiteral", digitLiteralCases},
		{"EscapedMeta", escapedMetaCases},
		{"Combined", combinedCases},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			for _, tc := range g.cases {
				t.Run(tc.pattern, func(t *testing.T) {
					// First: confirm regexp/syntax considers this valid.
					_, err := syntax.Parse(tc.pattern, syntax.Perl|syntax.ClassNL)
					if err != nil {
						t.Fatalf("regexp/syntax rejected %q (test bug): %v", tc.pattern, err)
					}

					data := []byte(tc.pattern)
					tree, n, llErr := matcher.Match(data)
					accepted := llErr == nil && n == len(data)

					if tc.accept && !accepted {
						msg := "grammar rejected"
						if llErr != nil {
							msg = fmt.Sprintf("grammar error: %v", llErr)
							if perr, ok := llErr.(langlang.ParsingError); ok {
								loc := tree.Location(perr.End)
								msg += fmt.Sprintf(" at %d:%d", loc.Line, loc.Column)
							}
						} else {
							msg = fmt.Sprintf("grammar consumed %d/%d bytes", n, len(data))
						}
						t.Errorf("expected accept for %q: %s", tc.pattern, msg)
					}
					if !tc.accept && accepted {
						t.Errorf("expected reject for %q but grammar accepted it (good news — maybe promote this case)", tc.pattern)
					}
				})
			}
		})
	}
}

// TestRegexCanonicalRoundTrip parses a large corpus of regex patterns
// with regexp/syntax, simplifies them to canonical form, and checks
// that the grammar accepts every canonical output.  This catches gaps
// between what regexp/syntax considers valid and what the PEG grammar
// can parse.
func TestRegexCanonicalRoundTrip(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	seeds := []string{
		// basics
		`a`, `abc`, `[a-z]`, `[A-Z]`, `.`, `\d`, `\w`, `\s`,

		// quantifiers
		`a*`, `a+`, `a?`, `a{3}`, `a{3,5}`, `a{3,}`,
		`a*?`, `a+?`, `a??`,

		// char classes
		`[abc]`, `[a-zA-Z0-9]`, `[^xyz]`, `[a-z0-9_]`,
		`[\d]`, `[\w]`, `[\s]`,
		`[[:alpha:]]`, `[[:digit:]]`, `[[:alnum:]]`,
		`[^aeiou]`, `[a-fA-F0-9]`,

		// groups and alternation
		`(abc)`, `(?:abc)`, `(a|b|c)`, `((a)b)`,
		`(foo|bar)+`, `(?:x|y|z)`,

		// anchors and escapes
		`^abc$`, `\bword\b`,
		`\+`, `\.`, `\\`, `\(`, `\)`,

		// dot forms (triggers (?-s:.) canonical)
		`.`, `.+`, `.*`, `.?`,

		// perl class combos
		`\d+`, `\w+`, `\s+`, `\D+`, `\W+`, `\S+`,
		`\d{2,4}`, `\w{1,}`,

		// unicode properties
		`\p{L}`, `\p{N}`, `\P{L}`,

		// posix classes
		`[[:alpha:]][[:alnum:]]*`,

		// nested / complex
		`(a(b(c)))`,
		`a*b+c?`,
		`(x|y|z){1,3}`,
		`[a-z]+@[a-z]+\.[a-z]+`,
		`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`,
		`(https?://)?[a-z]+(\.[a-z]+)+`,
		`\(\d{3}\)\s?\d{3}-\d{4}`,
		`[a-zA-Z_][a-zA-Z0-9_]*`,
		`"([^"\\]|\\.)*"`,
		`[a-z]+(\.[a-z]+)*`,
		`\d+\.\d+`,

		// flag groups
		`(?i:abc)`, `(?-s:.)`, `(?im:foo)`,

		// named group
		`(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`,

		// repeated groups
		`(ab)+`, `(a|b)*`, `(?:xy){2,5}`,

		// char class with escapes and ranges
		`[\d\s]+`, `[a-z\d]+`, `[\w-]+`,

		// quantifier on class
		`[a-z]{2,4}`, `[0-9]{1,3}`,

		// lazy on groups
		`(a+?b)`, `(.*?)`,

		// adjacent quantified atoms
		`a+b+c+`, `\d+\w*\s?`,

		// alternation with quantifiers
		`foo|bar|baz`,
		`(cat|dog|bird)+`,

		// real-world-ish
		`[A-Z][a-z]+`,
		`0x[0-9a-fA-F]+`,
		`[+-]?\d+(\.\d+)?`,
		`\w+@\w+\.\w+`,
	}

	var failures []string
	for _, seed := range seeds {
		re, err := syntax.Parse(seed, syntax.Perl|syntax.ClassNL)
		if err != nil {
			t.Fatalf("bad seed %q: %v", seed, err)
		}
		canonical := re.Simplify().String()

		data := []byte(canonical)
		_, n, llErr := matcher.Match(data)
		if llErr != nil || n != len(data) {
			failures = append(failures, fmt.Sprintf("seed %q → canonical %q", seed, canonical))
		}
	}

	if len(failures) > 0 {
		for _, f := range failures {
			t.Errorf("grammar rejected: %s", f)
		}
		t.Errorf("%d/%d canonical patterns rejected", len(failures), len(seeds))
	} else {
		t.Logf("all %d canonical round-trips accepted", len(seeds))
	}
}

// TestRegexSystematicGeneration builds regex patterns from components
// and checks that both regexp/syntax and the grammar accept them.
func TestRegexSystematicGeneration(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	atoms := []string{
		`a`, `z`, `1`, `0`,
		`.`,
		`\d`, `\D`, `\w`, `\W`, `\s`, `\S`,
		`\+`, `\.`, `\\`,
		`[a-z]`, `[A-Z]`, `[0-9]`, `[a-zA-Z]`, `[abc]`,
		`[^a-z]`, `[^0-9]`,
		`(a)`, `(?:a)`, `(ab)`,
	}

	quantifiers := []string{
		``, `*`, `+`, `?`, `{2}`, `{1,3}`, `{2,}`,
		`*?`, `+?`, `??`,
	}

	var failures []string
	total := 0
	for _, atom := range atoms {
		for _, q := range quantifiers {
			pattern := atom + q
			_, err := syntax.Parse(pattern, syntax.Perl|syntax.ClassNL)
			if err != nil {
				continue
			}
			total++

			data := []byte(pattern)
			_, n, llErr := matcher.Match(data)
			if llErr != nil || n != len(data) {
				failures = append(failures, pattern)
			}
		}
	}

	if len(failures) > 0 {
		for _, f := range failures {
			t.Errorf("grammar rejected %q", f)
		}
		t.Errorf("%d/%d generated patterns rejected", len(failures), total)
	} else {
		t.Logf("all %d generated atom+quantifier patterns accepted", total)
	}
}

// TestRegexConcatenationGeneration tests pairs of atoms concatenated
// together, exercising the Concatenation rule.
func TestRegexConcatenationGeneration(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	parts := []string{
		`a`, `\d`, `[a-z]`, `.`, `(a|b)`, `\w`, `\+`, `[0-9]`,
	}

	quantifiers := []string{``, `+`, `*`, `?`}

	var failures []string
	total := 0
	for _, a := range parts {
		for _, qa := range quantifiers {
			for _, b := range parts {
				for _, qb := range quantifiers {
					pattern := a + qa + b + qb
					_, err := syntax.Parse(pattern, syntax.Perl|syntax.ClassNL)
					if err != nil {
						continue
					}
					total++

					data := []byte(pattern)
					_, n, llErr := matcher.Match(data)
					if llErr != nil || n != len(data) {
						failures = append(failures, pattern)
					}
				}
			}
		}
	}

	if len(failures) > 0 {
		for _, f := range failures {
			t.Errorf("grammar rejected %q", f)
		}
		t.Errorf("%d/%d concatenation patterns rejected", len(failures), total)
	} else {
		t.Logf("all %d concatenation patterns accepted", total)
	}
}

// TestRegexAlternationGeneration tests patterns with alternation,
// including alternation inside groups and with quantifiers.
func TestRegexAlternationGeneration(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	atoms := []string{
		`a`, `\d`, `[a-z]`, `.`, `\w`, `[0-9]`, `\s`,
	}

	var failures []string
	total := 0

	for _, a := range atoms {
		for _, b := range atoms {
			// bare alternation: a|b
			pattern := a + `|` + b
			if tryParse(pattern) {
				total++
				if !grammarAccepts(matcher, pattern) {
					failures = append(failures, pattern)
				}
			}

			// grouped alternation: (a|b)
			pattern = `(` + a + `|` + b + `)`
			if tryParse(pattern) {
				total++
				if !grammarAccepts(matcher, pattern) {
					failures = append(failures, pattern)
				}
			}

			// grouped alternation with quantifier: (a|b)+
			for _, q := range []string{`*`, `+`, `?`} {
				pattern = `(` + a + `|` + b + `)` + q
				if tryParse(pattern) {
					total++
					if !grammarAccepts(matcher, pattern) {
						failures = append(failures, pattern)
					}
				}
			}
		}
	}

	if len(failures) > 0 {
		for _, f := range failures {
			t.Errorf("grammar rejected %q", f)
		}
		t.Errorf("%d/%d alternation patterns rejected", len(failures), total)
	} else {
		t.Logf("all %d alternation patterns accepted", total)
	}
}

// TestRegexCanonicalStress runs the canonical round-trip on the
// cartesian product of atoms, quantifiers, and wrappers.
func TestRegexCanonicalStress(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(t, err)

	// Patterns that exercise interesting regexp/syntax simplifications.
	patterns := []string{
		// \d expansion
		`\d`, `\d+`, `\d*`, `\d?`, `\d{2}`, `\d{2,4}`,
		// \w expansion
		`\w`, `\w+`, `\w*`,
		// \s expansion (produces [\t-\n\f-\r ])
		`\s`, `\s+`, `\s*`,
		// \D \W \S (negated classes)
		`\D+`, `\W+`, `\S+`,
		// dot (produces (?-s:.))
		`.`, `..`, `.+`, `.*`, `.?`, `.{2,5}`,
		// POSIX classes
		`[[:alpha:]]`, `[[:alpha:]]+`, `[[:digit:]]+`,
		`[[:alnum:]]+`, `[[:space:]]`,
		// anchored patterns
		`^\d+$`, `^\w+$`, `^[a-z]+$`,
		// complex canonical forms
		`\d{3}-\d{4}`,
		`\d{1,3}\.\d{1,3}`,
		`[a-z]+@[a-z]+`,
		`(a|b){2,4}`,
		`(\d+\.){3}\d+`,
		`[a-zA-Z_]\w*`,
		// nested groups
		`((a|b)(c|d))`,
		`(a(b(c(d))))`,
		// flag groups
		`(?i:foo)`, `(?-i:bar)`, `(?s:.)`,
		// non-capturing
		`(?:\d+)`, `(?:[a-z]+)`,
		// named groups
		`(?P<x>\d+)`,
		// character class combos
		`[\d\w]`, `[\s\S]`, `[a-z\d]`,
		`[^\d]`, `[^\s]`, `[^\w]`,
		// escaped specials
		`\.\*\+\?\|\(\)\[\]\{\}`,
		`\^\$\\`,
	}

	var failures []string
	for _, p := range patterns {
		re, err := syntax.Parse(p, syntax.Perl|syntax.ClassNL)
		if err != nil {
			t.Fatalf("bad pattern %q: %v", p, err)
		}
		canonical := re.Simplify().String()

		if !grammarAccepts(matcher, canonical) {
			failures = append(failures, fmt.Sprintf("pattern %q → canonical %q", p, canonical))
		}
	}

	if len(failures) > 0 {
		for _, f := range failures {
			t.Errorf("grammar rejected: %s", f)
		}
		t.Errorf("%d/%d canonical stress patterns rejected", len(failures), len(patterns))
	} else {
		t.Logf("all %d canonical stress patterns accepted", len(patterns))
	}
}

func tryParse(pattern string) bool {
	_, err := syntax.Parse(pattern, syntax.Perl|syntax.ClassNL)
	return err == nil
}

func grammarAccepts(matcher langlang.Matcher, pattern string) bool {
	data := []byte(pattern)
	_, n, err := matcher.Match(data)
	return err == nil && n == len(data)
}

func BenchmarkParser(b *testing.B) {
	matcher, err := langlang.MatcherFromFilePath(grammarPath)
	require.NoError(b, err)
	patterns := []struct {
		name    string
		pattern string
	}{
		{"simple-literal", "abc"},
		{"char-class", "[a-zA-Z]+"},
		{"quantifiers", "a{3,5}"},
		{"alternation", "foo|bar|baz"},
		{"groups", "(a|b)(c|d)"},
		{"complex", `[a-z]+(\.[a-z]+)*`},
		{"perl-escapes", `\d+\w*\s?`},
	}

	for _, tc := range patterns {
		data := []byte(tc.pattern)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				_, _, err := matcher.Match(data)
				if err != nil {
					b.Fatalf("match error on %s: %v", tc.name, err)
				}
			}
		})
	}
}

func BenchmarkRegexpSyntax(b *testing.B) {
	patterns := []struct {
		name    string
		pattern string
	}{
		{"simple-literal", "abc"},
		{"char-class", "[a-zA-Z]+"},
		{"quantifiers", "a{3,5}"},
		{"alternation", "foo|bar|baz"},
		{"groups", "(a|b)(c|d)"},
		{"complex", `[a-z]+(\.[a-z]+)*`},
		{"perl-escapes", `\d+\w*\s?`},
	}

	for _, tc := range patterns {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.pattern)))
			for i := 0; i < b.N; i++ {
				_, err := syntax.Parse(tc.pattern, syntax.Perl|syntax.ClassNL)
				if err != nil {
					b.Fatalf("parse error on %s: %v", tc.name, err)
				}
			}
		})
	}
}

