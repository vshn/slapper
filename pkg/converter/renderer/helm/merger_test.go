package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/slapper/pkg/converter/kcl"
	"github.com/vshn/slapper/pkg/servicebundle"
)

func TestMergeValues_EmptyEverything(t *testing.T) {
	got, err := MergeValues(servicebundle.HelmSource{}, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, got)
}

func TestMergeValues_StaticOnly(t *testing.T) {
	h := servicebundle.HelmSource{Values: map[string]any{
		"fullnameOverride": "pg",
		"cluster":          map[string]any{"instances": 1},
	}}
	got, err := MergeValues(h, nil)
	require.NoError(t, err)
	assert.Equal(t, "pg", got["fullnameOverride"])
	assert.Equal(t, map[string]any{"instances": 1}, got["cluster"])
}

func TestMergeValues_DeepCloneDoesNotMutateInput(t *testing.T) {
	cluster := map[string]any{"instances": 1}
	h := servicebundle.HelmSource{Values: map[string]any{"cluster": cluster}}
	vm := []servicebundle.ValueMappingItem{
		{ClaimPath: ".spec.parameters.size.replicas", Target: "cluster.instances"},
	}
	_, err := MergeValues(h, vm)
	require.NoError(t, err)
	assert.Equal(t, 1, cluster["instances"], "input map must not be mutated")
}

func TestMergeValues_MappingOverwritesStatic(t *testing.T) {
	h := servicebundle.HelmSource{Values: map[string]any{
		"cluster": map[string]any{"instances": 1},
	}}
	vm := []servicebundle.ValueMappingItem{
		{ClaimPath: ".spec.parameters.size.replicas", Target: "cluster.instances"},
	}
	got, err := MergeValues(h, vm)
	require.NoError(t, err)
	cluster := got["cluster"].(map[string]any)
	assert.Equal(t, kcl.RawKCL{Expr: "oxr.spec.parameters.size.replicas"}, cluster["instances"])
}

func TestMergeValues_MappingCreatesPath(t *testing.T) {
	h := servicebundle.HelmSource{Values: map[string]any{}}
	vm := []servicebundle.ValueMappingItem{
		{ClaimPath: ".spec.parameters.size.disk", Target: "cluster.storage.size"},
	}
	got, err := MergeValues(h, vm)
	require.NoError(t, err)
	cluster := got["cluster"].(map[string]any)
	storage := cluster["storage"].(map[string]any)
	assert.Equal(t, kcl.RawKCL{Expr: "oxr.spec.parameters.size.disk"}, storage["size"])
}

func TestMergeValues_TargetWithLeadingDotEquivalent(t *testing.T) {
	h := servicebundle.HelmSource{Values: map[string]any{}}
	vm := []servicebundle.ValueMappingItem{
		{ClaimPath: ".spec.x", Target: ".cluster.instances"},
	}
	got, err := MergeValues(h, vm)
	require.NoError(t, err)
	cluster := got["cluster"].(map[string]any)
	assert.Equal(t, kcl.RawKCL{Expr: "oxr.spec.x"}, cluster["instances"])
}

func TestMergeValues_ClaimPathMissingDotErrors(t *testing.T) {
	h := servicebundle.HelmSource{}
	vm := []servicebundle.ValueMappingItem{
		{ClaimPath: "spec.x", Target: "cluster.instances"},
	}
	_, err := MergeValues(h, vm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must start with '.'")
}

func TestMergeValues_MultipleMappings(t *testing.T) {
	h := servicebundle.HelmSource{Values: map[string]any{
		"fullnameOverride": "pg",
		"cluster": map[string]any{
			"instances": 1,
			"storage":   map[string]any{"size": "1Gi"},
		},
	}}
	vm := []servicebundle.ValueMappingItem{
		{ClaimPath: ".spec.parameters.size.replicas", Target: "cluster.instances"},
		{ClaimPath: ".spec.parameters.size.disk", Target: "cluster.storage.size"},
	}
	got, err := MergeValues(h, vm)
	require.NoError(t, err)
	cluster := got["cluster"].(map[string]any)
	assert.Equal(t, kcl.RawKCL{Expr: "oxr.spec.parameters.size.replicas"}, cluster["instances"])
	storage := cluster["storage"].(map[string]any)
	assert.Equal(t, kcl.RawKCL{Expr: "oxr.spec.parameters.size.disk"}, storage["size"])
	assert.Equal(t, "pg", got["fullnameOverride"])
}
