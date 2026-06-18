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
    inputTemplates:
      helm: templates/provisioning-helm.yaml
  - kind: networking
    function:
      name: xpkg.upbound.io/crossplane-contrib/function-kcl
      versionConstraint: ">=v0.10.0"
    inputFile: templates/networking.kcl
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
	require.Len(t, m.Steps, 2)

	prov := m.Steps[0]
	assert.Equal(t, servicebundle.StepProvisioning, prov.Kind)
	assert.Equal(t, "xpkg.upbound.io/crossplane-contrib/function-kcl", prov.Function.Name)
	assert.Equal(t, ">=v0.10.0", prov.Function.VersionConstraint)
	assert.Empty(t, prov.InputFile)
	assert.Equal(t, "templates/provisioning-helm.yaml", prov.InputTemplates["helm"])

	net := m.Steps[1]
	assert.Equal(t, servicebundle.StepNetworking, net.Kind)
	assert.Equal(t, "templates/networking.kcl", net.InputFile)
	assert.Empty(t, net.InputTemplates)

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
			{Kind: servicebundle.StepNetworking, Function: FunctionRef{Name: "f"}, InputFile: "a.kcl"},
			{Kind: servicebundle.StepNetworking, Function: FunctionRef{Name: "f"}, InputFile: "b.kcl"},
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
				Kind:           servicebundle.StepProvisioning,
				Function:       FunctionRef{Name: "function-kcl", VersionConstraint: ">=v0.10"},
				InputTemplates: map[string]string{"helm": "templates/provisioning-helm.yaml"},
			},
			{
				Kind:      servicebundle.StepNetworking,
				Function:  FunctionRef{Name: "function-kcl"},
				InputFile: "templates/networking.kcl",
			},
		},
	}
	require.NoError(t, m.Validate())
}

func TestManifest_Validate_ProvisioningWithInputTemplatesOK(t *testing.T) {
	m := &Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Steps: []StepEntry{{
			Kind:           servicebundle.StepProvisioning,
			Function:       FunctionRef{Name: "f"},
			InputTemplates: map[string]string{"helm": "templates/helm.yaml"},
		}},
	}
	assert.NoError(t, m.Validate())
}

func TestManifest_Validate_ProvisioningWithInputFileFails(t *testing.T) {
	m := &Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Steps: []StepEntry{{
			Kind:      servicebundle.StepProvisioning,
			Function:  FunctionRef{Name: "f"},
			InputFile: "templates/provisioning.kcl",
		}},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inputTemplates")
}

func TestManifest_Validate_NonProvisioningWithInputFileOK(t *testing.T) {
	m := &Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Steps: []StepEntry{{
			Kind:      servicebundle.StepNetworking,
			Function:  FunctionRef{Name: "f"},
			InputFile: "templates/networking.yaml",
		}},
	}
	assert.NoError(t, m.Validate())
}

func TestManifest_Validate_NonProvisioningWithInputTemplatesFails(t *testing.T) {
	m := &Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Steps: []StepEntry{{
			Kind:           servicebundle.StepBackup,
			Function:       FunctionRef{Name: "f"},
			InputTemplates: map[string]string{"helm": "x.yaml"},
		}},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inputFile")
}

func TestManifest_Validate_NeitherSetFails(t *testing.T) {
	m := &Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Steps: []StepEntry{{
			Kind:     servicebundle.StepProvisioning,
			Function: FunctionRef{Name: "f"},
		}},
	}
	err := m.Validate()
	require.Error(t, err)
}

func TestManifest_Validate_BothSetFails(t *testing.T) {
	m := &Manifest{
		APIVersion: ManifestAPIVersion,
		Kind:       ManifestKind,
		Steps: []StepEntry{{
			Kind:           servicebundle.StepProvisioning,
			Function:       FunctionRef{Name: "f"},
			InputFile:      "x",
			InputTemplates: map[string]string{"helm": "y"},
		}},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one")
}
