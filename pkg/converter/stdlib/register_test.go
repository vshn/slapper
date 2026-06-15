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
	pipeline.ResetForTest()            // helper to be added — see Step 4
	pipeline.RegisterDefaultsForTest() // helper to be added — see Step 4

	beforeR, ok := pipeline.Get(servicebundle.StepProvisioning)
	require.True(t, ok, "in-tree dummy should be registered")
	_ = beforeR

	m, files, err := Load(context.Background(), LocalSource("testdata"))
	require.NoError(t, err)

	require.NoError(t, RegisterAll(m, files))

	afterR, ok := pipeline.Get(servicebundle.StepProvisioning)
	require.True(t, ok)
	_, isStdlib := afterR.(*stdlibRenderer)
	assert.True(t, isStdlib, "stdlib renderer should override dummy")
}

func TestRegisterAll_RegistersAllKinds(t *testing.T) {
	pipeline.ResetForTest()
	m, files, err := Load(context.Background(), LocalSource("testdata"))
	require.NoError(t, err)
	require.NoError(t, RegisterAll(m, files))

	for _, kind := range []servicebundle.PipelineStepKind{
		servicebundle.StepProvisioning,
		servicebundle.StepNetworking,
		servicebundle.StepBackup,
		servicebundle.StepMonitoring,
		servicebundle.StepMaintenance,
	} {
		_, ok := pipeline.Get(kind)
		assert.True(t, ok, "kind %s not registered", kind)
	}
}
