package extract

import (
	"go/format"
	"strings"
	"testing"
)

func TestRenderFileJen(t *testing.T) {
	nameIDs := []NameIDEntry{
		{Name: "Value", ID: 0},
		{Name: "Object", ID: 1},
	}
	structs := []StructInfo{{
		Name: "JSONValue",
		Fields: []FieldInfo{
			{
				GoName: "Object", LLTag: "Object", Kind: FieldOptional,
				GoType: "*JSONObject", NameID: 1,
			},
		},
	}}
	rules := map[string]RuleInfo{
		"Value":  {Kind: RuleChoice, Choices: []string{"Object"}},
		"Object": {Kind: RuleSequence},
	}

	output, err := RenderFileJen("example", "test.peg", nameIDs, structs, rules)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"package example",
		"DO NOT EDIT",
		"_nameID_Value",
		"_nameID_Object",
		"ExtractJSONValue",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in output:\n%s", want, output)
		}
	}
}

func TestEmitChoiceFunctionJen(t *testing.T) {
	si := StructInfo{
		Name: "JSONValue",
		Fields: []FieldInfo{
			{GoName: "Object", LLTag: "Object", Kind: FieldOptional, GoType: "*JSONObject", NameID: 1},
			{GoName: "String", LLTag: "String", Kind: FieldOptional, GoType: "*string", NameID: 3},
		},
	}
	rules := map[string]RuleInfo{
		"Value":  {Name: "Value", Kind: RuleChoice, NameID: 0, Choices: []string{"Object", "String"}},
		"Object": {Name: "Object", Kind: RuleSequence, NameID: 1},
		"String": {Name: "String", Kind: RuleLeaf, NameID: 3},
	}

	nameIDs := []NameIDEntry{
		{Name: "Object", ID: 1},
		{Name: "String", ID: 3},
	}

	output, err := RenderFileJen("test", "test.peg", nameIDs, []StructInfo{si}, rules)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"_nameID_Object",
		"_nameID_String",
		"ExtractJSONValue",
		"IsNamed",
		"*tree",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in output:\n%s", want, output)
		}
	}
}

func TestEmitSequenceFunctionJen(t *testing.T) {
	si := StructInfo{
		Name: "JSONMember",
		Fields: []FieldInfo{
			{GoName: "Key", LLTag: "String", Kind: FieldText, GoType: "string", NameID: 3},
			{GoName: "Value", LLTag: "Value", Kind: FieldNamedRule, GoType: "JSONValue", NameID: 0},
		},
	}
	rules := map[string]RuleInfo{
		"Member": {
			Name: "Member", Kind: RuleSequence, NameID: 5,
			Children: []RuleChild{
				{RuleName: "String", Index: 0},
				{IsLiteral: true, Index: 1},
				{RuleName: "Value", Index: 2},
			},
		},
	}

	nameIDs := []NameIDEntry{
		{Name: "String", ID: 3},
		{Name: "Value", ID: 0},
	}

	output, err := RenderFileJen("test", "test.peg", nameIDs, []StructInfo{si}, rules)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"NodeType_Node",
		"Text(",
		"NameID(cid)",
		"_nameID_String",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in output:\n%s", want, output)
		}
	}
}

// jsonFixtures returns shared test data for the JSON extract example.
func jsonFixtures() ([]NameIDEntry, []StructInfo, map[string]RuleInfo) {
	nameIDs := []NameIDEntry{
		{Name: "Value", ID: 0},
		{Name: "Object", ID: 1},
		{Name: "Array", ID: 2},
		{Name: "String", ID: 3},
		{Name: "Number", ID: 4},
		{Name: "Member", ID: 5},
	}

	structs := []StructInfo{
		{
			Name: "JSONValue",
			Fields: []FieldInfo{
				{GoName: "Object", LLTag: "Object", Kind: FieldOptional, GoType: "*JSONObject", NameID: 1},
				{GoName: "Array", LLTag: "Array", Kind: FieldOptional, GoType: "*JSONArray", NameID: 2},
				{GoName: "String", LLTag: "String", Kind: FieldOptional, GoType: "*string", NameID: 3},
				{GoName: "Number", LLTag: "Number", Kind: FieldOptional, GoType: "*string", NameID: 4},
			},
		},
		{
			Name: "JSONObject",
			Fields: []FieldInfo{
				{GoName: "Members", LLTag: "Member", Kind: FieldSlice, GoType: "[]JSONMember", ElemType: "JSONMember", NameID: 5},
			},
		},
		{
			Name: "JSONMember",
			Fields: []FieldInfo{
				{GoName: "Key", LLTag: "String", Kind: FieldText, GoType: "string", NameID: 3},
				{GoName: "Value", LLTag: "Value", Kind: FieldNamedRule, GoType: "JSONValue", NameID: 0},
			},
		},
	}

	rules := map[string]RuleInfo{
		"Value":  {Name: "Value", Kind: RuleChoice, NameID: 0, Choices: []string{"Object", "Array", "String", "Number"}},
		"Object": {Name: "Object", Kind: RuleSequence, NameID: 1},
		"Array":  {Name: "Array", Kind: RuleSequence, NameID: 2},
		"String": {Name: "String", Kind: RuleLeaf, NameID: 3},
		"Number": {Name: "Number", Kind: RuleLeaf, NameID: 4},
		"Member": {Name: "Member", Kind: RuleSequence, NameID: 5},
	}

	return nameIDs, structs, rules
}

func TestJenMatchesText(t *testing.T) {
	nameIDs, structs, rules := jsonFixtures()

	textOut, err := RenderFile("testpkg", "test.peg", nameIDs, structs, rules)
	if err != nil {
		t.Fatalf("text emitter: %v", err)
	}

	jenOut, err := RenderFileJen("testpkg", "test.peg", nameIDs, structs, rules)
	if err != nil {
		t.Fatalf("jen emitter: %v", err)
	}

	// Normalize both through gofmt for fair comparison
	textFormatted, err := format.Source([]byte(textOut))
	if err != nil {
		t.Fatalf("format text output: %v", err)
	}
	jenFormatted, err := format.Source([]byte(jenOut))
	if err != nil {
		t.Fatalf("format jen output: %v", err)
	}

	// Normalize: strip comments, blank-only lines, and the var _ = fmt.Errorf guard
	textBody := normalizeSource(string(textFormatted))
	jenBody := normalizeSource(string(jenFormatted))

	if textBody != jenBody {
		t.Errorf("outputs differ.\n\n--- text emitter ---\n%s\n\n--- jen emitter ---\n%s", textBody, jenBody)
	}
}

func BenchmarkRenderFileText(b *testing.B) {
	nameIDs, structs, rules := jsonFixtures()
	for b.Loop() {
		_, err := RenderFile("testpkg", "test.peg", nameIDs, structs, rules)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderFileJen(b *testing.B) {
	nameIDs, structs, rules := jsonFixtures()
	for b.Loop() {
		_, err := RenderFileJen("testpkg", "test.peg", nameIDs, structs, rules)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func normalizeSource(src string) string {
	var lines []string
	for line := range strings.SplitSeq(src, "\n") {
		// skip comments, blank lines, and the import guard
		if strings.HasPrefix(line, "//") {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) == "var _ = fmt.Errorf" {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
