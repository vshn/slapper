package kcl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmit_Nil(t *testing.T) {
	got, err := Emit(nil)
	require.NoError(t, err)
	assert.Equal(t, "None", got)
}

func TestEmit_Bool(t *testing.T) {
	got, err := Emit(true)
	require.NoError(t, err)
	assert.Equal(t, "True", got)

	got, err = Emit(false)
	require.NoError(t, err)
	assert.Equal(t, "False", got)
}

func TestEmit_Int(t *testing.T) {
	cases := []any{1, int64(42), int32(-7)}
	expected := []string{"1", "42", "-7"}
	for i, c := range cases {
		got, err := Emit(c)
		require.NoError(t, err)
		assert.Equal(t, expected[i], got)
	}
}

func TestEmit_Float(t *testing.T) {
	got, err := Emit(1.5)
	require.NoError(t, err)
	assert.Equal(t, "1.5", got)
}

func TestEmit_StringPlain(t *testing.T) {
	got, err := Emit("hello")
	require.NoError(t, err)
	assert.Equal(t, `"hello"`, got)
}

func TestEmit_StringEscapes(t *testing.T) {
	got, err := Emit(`a\b"c` + "\n\t")
	require.NoError(t, err)
	assert.Equal(t, `"a\\b\"c\n\t"`, got)
}

func TestEmit_MapAlphabeticalKeys(t *testing.T) {
	got, err := Emit(map[string]any{"b": 2, "a": 1, "c": 3})
	require.NoError(t, err)
	assert.Equal(t, "{\n    a = 1\n    b = 2\n    c = 3\n}", got)
}

// Reserved KCL keywords are emitted with $-prefix escape, not quoted.
func TestEmit_MapReservedKeyDollarEscaped(t *testing.T) {
	got, err := Emit(map[string]any{"if": 1, "x": 2})
	require.NoError(t, err)
	assert.Equal(t, "{\n    $if = 1\n    x = 2\n}", got)
}

func TestEmit_MapInvalidIdentifierKeyQuoted(t *testing.T) {
	got, err := Emit(map[string]any{"with-dash": 1, "0num": 2})
	require.NoError(t, err)
	assert.Equal(t, "{\n    \"0num\" = 2\n    \"with-dash\" = 1\n}", got)
}

func TestEmit_List(t *testing.T) {
	got, err := Emit([]any{1, "x", true})
	require.NoError(t, err)
	assert.Equal(t, "[\n    1\n    \"x\"\n    True\n]", got)
}

func TestEmit_NestedMapAndList(t *testing.T) {
	in := map[string]any{
		"cluster": map[string]any{
			"instances": 1,
			"storage":   map[string]any{"size": "1Gi"},
		},
		"items": []any{"a", "b"},
	}
	got, err := Emit(in)
	require.NoError(t, err)
	want := "{\n" +
		"    cluster = {\n" +
		"        instances = 1\n" +
		"        storage = {\n" +
		"            size = \"1Gi\"\n" +
		"        }\n" +
		"    }\n" +
		"    items = [\n" +
		"        \"a\"\n" +
		"        \"b\"\n" +
		"    ]\n" +
		"}"
	assert.Equal(t, want, got)
}

func TestEmit_RefPassthrough(t *testing.T) {
	got, err := Emit(Ref{Expr: "oxr.spec.parameters.size.replicas"})
	require.NoError(t, err)
	assert.Equal(t, "oxr.spec.parameters.size.replicas", got)
}

func TestEmit_RefInsideMap(t *testing.T) {
	in := map[string]any{"instances": Ref{Expr: "oxr.spec.x"}}
	got, err := Emit(in)
	require.NoError(t, err)
	assert.Equal(t, "{\n    instances = oxr.spec.x\n}", got)
}

// Empty containers stay one-line via post-process — see Emit comment.
func TestEmit_EmptyMap(t *testing.T) {
	got, err := Emit(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, `{}`, got)
}

func TestEmit_EmptyList(t *testing.T) {
	got, err := Emit([]any{})
	require.NoError(t, err)
	assert.Equal(t, `[]`, got)
}
