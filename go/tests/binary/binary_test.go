package binary

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTripPrefixedString(t *testing.T) {
	original := PrefixedString{Data: []byte("hello")}
	encoded, err := EncodePrefixedString(&original)
	require.NoError(t, err)

	decoded, n, err := DecodePrefixedString(encoded)
	require.NoError(t, err)
	assert.Equal(t, len(encoded), n)
	assert.Equal(t, original.Data, decoded.Data)
}

func TestRoundTripSkuRecord(t *testing.T) {
	original := SkuRecord{
		Genre:        PrefixedString{Data: []byte("zettel")},
		ObjectId:     PrefixedString{Data: []byte("abc123")},
		TypeId:       PrefixedString{Data: []byte("typ")},
		Tags:         PrefixedStringList{Items: []PrefixedString{{Data: []byte("tag1")}, {Data: []byte("tag2")}}},
		TagsImplicit: PrefixedStringList{Items: []PrefixedString{}},
		BlobDigest:   PrefixedString{Data: []byte("sha256:deadbeef")},
		Description:  PrefixedString{Data: []byte("a test record")},
	}
	encoded, err := EncodeSkuRecord(&original)
	require.NoError(t, err)

	decoded, n, err := DecodeSkuRecord(encoded)
	require.NoError(t, err)
	assert.Equal(t, len(encoded), n)
	assert.Equal(t, original, decoded)
}

func TestGoldenBytes(t *testing.T) {
	ps := PrefixedString{Data: []byte("hi")}
	encoded, err := EncodePrefixedString(&ps)
	require.NoError(t, err)

	// u32le(2) = 0x02 0x00 0x00 0x00, then "hi"
	expected := []byte{0x02, 0x00, 0x00, 0x00, 'h', 'i'}
	assert.Equal(t, expected, encoded)
}

func TestDecodeShortInput(t *testing.T) {
	_, _, err := DecodePrefixedString([]byte{0x05, 0x00})
	assert.Error(t, err)
}

func TestEmptyList(t *testing.T) {
	original := PrefixedStringList{Items: []PrefixedString{}}
	encoded, err := EncodePrefixedStringList(&original)
	require.NoError(t, err)

	decoded, _, err := DecodePrefixedStringList(encoded)
	require.NoError(t, err)
	assert.Equal(t, 0, len(decoded.Items))
}

func TestEmptyString(t *testing.T) {
	original := PrefixedString{Data: []byte{}}
	encoded, err := EncodePrefixedString(&original)
	require.NoError(t, err)

	// u32le(0) = 0x00 0x00 0x00 0x00, no data
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, encoded)

	decoded, n, err := DecodePrefixedString(encoded)
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte{}, decoded.Data)
}
