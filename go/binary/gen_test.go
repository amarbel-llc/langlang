package binary

import (
	"testing"

	langlang "github.com/clarete/langlang/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseGrammar(t *testing.T, input string) *langlang.GrammarNode {
	t.Helper()
	parser := langlang.NewGrammarParser([]byte(input))
	ast, err := parser.Parse()
	require.NoError(t, err)
	grammar, ok := ast.(*langlang.GrammarNode)
	require.True(t, ok)
	require.Empty(t, grammar.Errors)
	return grammar
}

func TestGeneratePrefixedString(t *testing.T) {
	grammar := parseGrammar(t, "prefixed_string <- len:u32le data:bytes(len)")

	output, err := Generate(grammar, Options{PackageName: "wire"})
	require.NoError(t, err)

	assert.Contains(t, output, "type PrefixedString struct")
	assert.Contains(t, output, "Data []byte")
	assert.NotContains(t, output, "Len ") // len is a length field, not in struct
	assert.Contains(t, output, "func DecodePrefixedString(")
	assert.Contains(t, output, "func EncodePrefixedString(")
	assert.Contains(t, output, "func SizePrefixedString(")
	assert.Contains(t, output, "binary.LittleEndian")
}

func TestGenerateSkuRecord(t *testing.T) {
	grammar := parseGrammar(t, `sku_record <- genre:prefixed_string object_id:prefixed_string type_id:prefixed_string tags:prefixed_string_list tags_implicit:prefixed_string_list blob_digest:prefixed_string description:prefixed_string
prefixed_string <- len:u32le data:bytes(len)
prefixed_string_list <- count:u32le items:prefixed_string{count}`)

	output, err := Generate(grammar, Options{PackageName: "wire"})
	require.NoError(t, err)

	assert.Contains(t, output, "type SkuRecord struct")
	assert.Contains(t, output, "PrefixedString")
	assert.Contains(t, output, "PrefixedStringList")
	assert.Contains(t, output, "type PrefixedStringList struct")
	assert.Contains(t, output, "Items []PrefixedString")
	assert.Contains(t, output, "DecodeSkuRecord")
	assert.Contains(t, output, "EncodeSkuRecord")
	assert.Contains(t, output, "SizeSkuRecord")
}

func TestToGoName(t *testing.T) {
	assert.Equal(t, "PrefixedString", toGoName("prefixed_string"))
	assert.Equal(t, "U32le", toGoName("u32le"))
	assert.Equal(t, "A", toGoName("a"))
	assert.Equal(t, "SkuRecord", toGoName("sku_record"))
}
