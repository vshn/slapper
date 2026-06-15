package stdlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/slapper/pkg/servicebundle"
)

func makeManifest() *Manifest {
	return &Manifest{
		APIVersion: "slapper.appslap.io/v1alpha1",
		Kind:       "Stdlib",
		Steps: []StepEntry{
			{Kind: servicebundle.StepProvisioning, Function: FunctionRef{Name: "fn-kcl", VersionConstraint: ">=v0.10"}},
			{Kind: servicebundle.StepNetworking, Function: FunctionRef{Name: "fn-kcl", VersionConstraint: ">=v0.10"}},
			{Kind: servicebundle.StepMonitoring, Function: FunctionRef{Name: "fn-py", VersionConstraint: ">=v0.2"}},
		},
	}
}

func TestBuildDependencies_DedupesIdentical(t *testing.T) {
	used := []servicebundle.PipelineStep{
		{Kind: servicebundle.StepProvisioning},
		{Kind: servicebundle.StepNetworking},
	}
	deps, err := BuildDependencies(makeManifest(), used)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "fn-kcl", deps[0].Function)
	assert.Equal(t, ">=v0.10", deps[0].Version)
}

func TestBuildDependencies_IncludesAllUsedKinds(t *testing.T) {
	used := []servicebundle.PipelineStep{
		{Kind: servicebundle.StepProvisioning},
		{Kind: servicebundle.StepMonitoring},
	}
	deps, err := BuildDependencies(makeManifest(), used)
	require.NoError(t, err)
	require.Len(t, deps, 2)
}

func TestBuildDependencies_ConflictingConstraints(t *testing.T) {
	m := makeManifest()
	m.Steps = append(m.Steps, StepEntry{
		Kind:     servicebundle.StepBackup,
		Function: FunctionRef{Name: "fn-kcl", VersionConstraint: "<v0.9"},
	})
	used := []servicebundle.PipelineStep{
		{Kind: servicebundle.StepProvisioning},
		{Kind: servicebundle.StepBackup},
	}
	_, err := BuildDependencies(m, used)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting version constraints")
	assert.Contains(t, err.Error(), "fn-kcl")
}

func TestBuildDependencies_IncludesCustomStep(t *testing.T) {
	m := makeManifest()
	used := []servicebundle.PipelineStep{
		{Kind: servicebundle.StepCustom, Spec: &servicebundle.CustomStep{
			Function: servicebundle.FuncRef{Name: "ghcr.io/x/fn-custom", VersionConstraint: ">=v1"},
		}},
	}
	deps, err := BuildDependencies(m, used)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "ghcr.io/x/fn-custom", deps[0].Function)
	assert.Equal(t, ">=v1", deps[0].Version)
}

func TestBuildDependencies_UnknownStepKindError(t *testing.T) {
	m := makeManifest()
	used := []servicebundle.PipelineStep{{Kind: servicebundle.StepBackup}}
	_, err := BuildDependencies(m, used)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup")
}
