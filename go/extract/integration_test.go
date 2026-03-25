package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationJSON(t *testing.T) {
	dir := t.TempDir()
	grammarPath := jsonGrammarPath()

	src := filepath.Join(dir, "json_types.go")
	err := os.WriteFile(src, []byte(`package json

type JSONValue struct {
	Object *JSONObject `+"`"+`ll:"Object"`+"`"+`
	Array  *JSONArray  `+"`"+`ll:"Array"`+"`"+`
	String *string     `+"`"+`ll:"String"`+"`"+`
	Number *string     `+"`"+`ll:"Number"`+"`"+`
}

type JSONObject struct {
	Members []JSONMember `+"`"+`ll:"Member"`+"`"+`
}

type JSONMember struct {
	Key   string    `+"`"+`ll:"String"`+"`"+`
	Value JSONValue `+"`"+`ll:"Value"`+"`"+`
}

type JSONArray struct {
	Items []JSONValue `+"`"+`ll:"Value"`+"`"+`
}
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = Generate(src, grammarPath)
	if err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(dir, "json_types_extract.go")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}

	output := string(data)

	checks := []string{
		"DO NOT EDIT",
		"package json",
		"ExtractJSONValue",
		"ExtractJSONObject",
		"ExtractJSONMember",
		"ExtractJSONArray",
		"NodeType_Node",
		"NodeType_String",
		"t.Name(",
		"t.Type(",
		"t.Text(",
		"t.Child(",
		"t.Visit(",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q", check)
		}
	}

	t.Logf("Generated output:\n%s", output)
}
