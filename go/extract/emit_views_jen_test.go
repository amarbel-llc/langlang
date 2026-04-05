package extract

import (
	"go/format"
	"strings"
	"testing"
)

func TestRenderViewsFileJen(t *testing.T) {
	rules := map[string]RuleInfo{
		"Root": {Name: "Root", Kind: RuleLeaf, NameID: 0},
		"Item": {Name: "Item", Kind: RuleLeaf, NameID: 1},
	}

	output, err := RenderViewsFileJen("mypkg", "test.peg", rules, "Root")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"package mypkg",
		"DO NOT EDIT",
		"_nameID_Root",
		"_nameID_Item",
		"type Root struct",
		"type item_view struct",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in output:\n%s", want, output)
		}
	}
}

func TestViewsJenMatchesText(t *testing.T) {
	// Use a grammar with all rule kinds: sequence, choice (with literals),
	// repeat, alias, optional, and leaf.
	rules := map[string]RuleInfo{
		"JSON": {
			Name: "JSON", Kind: RuleSequence, NameID: 0,
			Children: []RuleChild{
				{RuleName: "Value", Index: 0},
				{RuleName: "EOF", Index: 1},
			},
		},
		"Value": {
			Name: "Value", Kind: RuleChoice, NameID: 1,
			Choices: []string{"Object", "Array", "String", "Number", "lit:true", "lit:false", "lit:null"},
		},
		"Object": {
			Name: "Object", Kind: RuleSequence, NameID: 2,
			Children: []RuleChild{
				{IsLiteral: true, Index: 0},
				{RuleName: "Member", Index: 1},
				{RuleName: "Member", Index: 2},
				{IsLiteral: true, Index: 3},
			},
		},
		"Array": {
			Name: "Array", Kind: RuleSequence, NameID: 3,
			Children: []RuleChild{
				{IsLiteral: true, Index: 0},
				{RuleName: "Value", Index: 1},
				{RuleName: "Value", Index: 2},
				{IsLiteral: true, Index: 3},
			},
		},
		"Member": {
			Name: "Member", Kind: RuleSequence, NameID: 4,
			Children: []RuleChild{
				{RuleName: "String", Index: 0},
				{IsLiteral: true, Index: 1},
				{RuleName: "Value", Index: 2},
			},
		},
		"String": {Name: "String", Kind: RuleLeaf, NameID: 5},
		"Number": {Name: "Number", Kind: RuleLeaf, NameID: 6},
		"EOF":    {Name: "EOF", Kind: RuleLeaf, NameID: 7},
		"Char":   {Name: "Char", Kind: RuleLeaf, NameID: 8},
	}

	textOut, err := RenderViewsFile("testpkg", "test.peg", rules, "JSON")
	if err != nil {
		t.Fatalf("text emitter: %v", err)
	}

	jenOut, err := RenderViewsFileJen("testpkg", "test.peg", rules, "JSON")
	if err != nil {
		t.Fatalf("jen emitter: %v", err)
	}

	textFormatted, err := format.Source([]byte(textOut))
	if err != nil {
		t.Fatalf("format text: %v", err)
	}
	jenFormatted, err := format.Source([]byte(jenOut))
	if err != nil {
		t.Fatalf("format jen: %v\nsource:\n%s", err, jenOut)
	}

	textBody := normalizeSource(string(textFormatted))
	jenBody := normalizeSource(string(jenFormatted))

	if textBody != jenBody {
		t.Errorf("outputs differ.\n\n--- text emitter ---\n%s\n\n--- jen emitter ---\n%s", textBody, jenBody)
	}
}
