package stdlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/vshn/slapper/pkg/servicebundle"
)

func TestManifest_UnmarshalRoundTrip(t *testing.T) {
	raw := []byte(`
apiVersion: slapper.appslap.io/v1alpha1
kind: Stdlib
metadata:
  name: vshn-stdlib
  version: 0.1.0
steps:
  - kind: provisioning
    function:
      name: xpkg.upbound.io/crossplane-contrib/function-kcl
      versionConstraint: ">=v0.10.0"
    inputFile: templates/provisioning.kcl
schemaFragments:
  size: schemas/size.yaml
plans:
  schema: schemas/plans.yaml
`)

	var m Manifest
	require.NoError(t, yaml.Unmarshal(raw, &m))

	assert.Equal(t, "slapper.appslap.io/v1alpha1", m.APIVersion)
	assert.Equal(t, "Stdlib", m.Kind)
	assert.Equal(t, "vshn-stdlib", m.Metadata.Name)
	assert.Equal(t, "0.1.0", m.Metadata.Version)
	require.Len(t, m.Steps, 1)
	assert.Equal(t, servicebundle.StepProvisioning, m.Steps[0].Kind)
	assert.Equal(t, "xpkg.upbound.io/crossplane-contrib/function-kcl", m.Steps[0].Function.Name)
	assert.Equal(t, ">=v0.10.0", m.Steps[0].Function.VersionConstraint)
	assert.Equal(t, "templates/provisioning.kcl", m.Steps[0].InputFile)
	assert.Equal(t, "schemas/size.yaml", m.SchemaFragments["size"])
	require.NotNil(t, m.Plans)
	assert.Equal(t, "schemas/plans.yaml", m.Plans.Schema)
}

func TestManifest_Validate_UnknownAPIVersion(t *testing.T) {
	m := Manifest{APIVersion: "wrong/v1", Kind: "Stdlib"}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported stdlib apiVersion")
}

func TestManifest_Validate_UnknownKind(t *testing.T) {
	m := Manifest{APIVersion: "slapper.appslap.io/v1alpha1", Kind: "NotStdlib"}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported stdlib kind")
}

func TestManifest_Validate_DuplicateStepKind(t *testing.T) {
	m := Manifest{
		APIVersion: "slapper.appslap.io/v1alpha1",
		Kind:       "Stdlib",
		Steps: []StepEntry{
			{Kind: servicebundle.StepProvisioning, InputFile: "a.kcl"},
			{Kind: servicebundle.StepProvisioning, InputFile: "b.kcl"},
		},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate step kind")
}

func TestManifest_Validate_OK(t *testing.T) {
	m := Manifest{
		APIVersion: "slapper.appslap.io/v1alpha1",
		Kind:       "Stdlib",
		Steps: []StepEntry{
			{
				Kind: servicebundle.StepProvisioning, InputFile: "a.kcl",
				Function: FunctionRef{Name: "function-kcl", VersionConstraint: ">=v0.10"},
			},
		},
	}
	require.NoError(t, m.Validate())
}
