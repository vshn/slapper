package kcl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmit_RefPassthrough(t *testing.T) {
	got, err := Emit(RawKCL{Expr: "oxr.spec.parameters.size.replicas"})
	require.NoError(t, err)
	assert.Equal(t, "oxr.spec.parameters.size.replicas", got)
}

func TestEmit_RefInsideMap(t *testing.T) {
	in := map[string]any{"instances": RawKCL{Expr: "oxr.spec.x"}}
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
