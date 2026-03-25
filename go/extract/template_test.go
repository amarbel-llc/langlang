package extract

import (
	"strings"
	"testing"
)

func TestRenderFile(t *testing.T) {
	structs := []StructInfo{{
		Name: "JSONValue",
		Fields: []FieldInfo{
			{GoName: "Object", LLTag: "Object", Kind: FieldOptional,
				GoType: "*JSONObject", NameID: 1},
		},
	}}
	rules := map[string]RuleInfo{
		"Value":  {Kind: RuleChoice, Choices: []string{"Object"}},
		"Object": {Kind: RuleSequence},
	}

	output, err := RenderFile("example", "test.peg", structs, rules)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "package example") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(output, "DO NOT EDIT") {
		t.Error("missing generated code header")
	}
	if !strings.Contains(output, "ExtractJSONValue") {
		t.Error("missing ExtractJSONValue function")
	}
}
