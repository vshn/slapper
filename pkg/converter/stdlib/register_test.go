package stdlib

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/slapper/pkg/converter/pipeline"
	"github.com/vshn/slapper/pkg/servicebundle"
)

func TestRegisterAll_OverridesInTreeDummies(t *testing.T) {
	pipeline.ResetForTest()
	pipeline.RegisterDefaultsForTest()

	beforeR, ok := pipeline.Get(servicebundle.StepProvisioning)
	require.True(t, ok, "in-tree dummy should be registered")
	_ = beforeR

	m, files, err := Load(context.Background(), LocalSource("testdata"))
	require.NoError(t, err)

	bundle := minimalHelmBundle()
	require.NoError(t, RegisterAll(m, files, bundle))

	afterR, ok := pipeline.Get(servicebundle.StepProvisioning)
	require.True(t, ok)
	_, isStdlib := afterR.(*stdlibRenderer)
	assert.True(t, isStdlib, "stdlib renderer should override dummy")
}

func TestRegisterAll_RegistersAllKinds(t *testing.T) {
	pipeline.ResetForTest()
	pipeline.RegisterDefaultsForTest()

	m, files, err := Load(context.Background(), LocalSource("testdata"))
	require.NoError(t, err)

	bundle := minimalHelmBundle()
	require.NoError(t, RegisterAll(m, files, bundle))

	for _, s := range m.Steps {
		_, ok := pipeline.Get(s.Kind)
		assert.True(t, ok, "kind %s not registered", s.Kind)
	}
}
