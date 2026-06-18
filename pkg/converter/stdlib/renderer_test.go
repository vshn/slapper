package stdlib

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/slapper/pkg/servicebundle"
)

func minimalHelmBundle() *servicebundle.ServiceBundle {
	return &servicebundle.ServiceBundle{
		Meta: servicebundle.Meta{Name: "pg", Author: "vshn", Version: "0.1.0"},
		Renderer: &servicebundle.Renderer{
			Type: servicebundle.RendererTypeHelm,
			Spec: &servicebundle.HelmSource{
				Repository: "https://charts.cnpg.io/",
				Chart:      "cluster",
				Version:    "0.4.0",
				Values:     map[string]any{"fullnameOverride": "pg"},
			},
		},
	}
}

// existing TestStdlibRenderer_Render is removed and replaced; new testdata
// directory pkg/converter/stdlib/testdata uses inputTemplates for provisioning
// (covered by Task 12).

func TestStdlibRenderer_Render_Provisioning_FromInputTemplate(t *testing.T) {
	files := fstest.MapFS{
		"tmpl.yaml": &fstest.MapFile{Data: []byte(`
apiVersion: v1
kind: X
spec:
  repository:
    $slapperRef: renderer.spec.repository
  values:
    $slapperRef: renderer.spec.values
`)},
	}
	entry := StepEntry{
		Kind:           servicebundle.StepProvisioning,
		Function:       FunctionRef{Name: "function-kcl"},
		InputTemplates: map[string]string{"helm": "tmpl.yaml"},
	}
	r := newRenderer(entry, files, minimalHelmBundle())

	out, err := r.Render(servicebundle.PipelineStep{Kind: servicebundle.StepProvisioning}, 0)
	require.NoError(t, err)

	assert.Equal(t, "provisioning-0", out["step"])
	input, ok := out["input"].(map[string]any)
	require.True(t, ok)
	spec, ok := input["spec"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://charts.cnpg.io/", spec["repository"])
	assert.Equal(t, map[string]any{"fullnameOverride": "pg"}, spec["values"])
}

func TestStdlibRenderer_Render_Provisioning_MissingTypeTemplateErrors(t *testing.T) {
	entry := StepEntry{
		Kind:           servicebundle.StepProvisioning,
		Function:       FunctionRef{Name: "function-kcl"},
		InputTemplates: map[string]string{"plain_manifests": "x.yaml"},
	}
	r := newRenderer(entry, fstest.MapFS{}, minimalHelmBundle())
	_, err := r.Render(servicebundle.PipelineStep{Kind: servicebundle.StepProvisioning}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "helm")
}

func TestStdlibRenderer_Render_NonProvisioning_UsesInputFile(t *testing.T) {
	files := fstest.MapFS{
		"networking.yaml": &fstest.MapFile{Data: []byte(`apiVersion: v1
kind: KCLInput
`)},
	}
	entry := StepEntry{
		Kind:      servicebundle.StepNetworking,
		Function:  FunctionRef{Name: "function-kcl"},
		InputFile: "networking.yaml",
	}
	r := newRenderer(entry, files, minimalHelmBundle())

	out, err := r.Render(servicebundle.PipelineStep{Kind: servicebundle.StepNetworking}, 0)
	require.NoError(t, err)
	assert.Equal(t, "networking-0", out["step"])
}

func TestStdlibRenderer_Render_ValuesKCLEndToEnd(t *testing.T) {
	bundle := minimalHelmBundle()
	bundle.Renderer.Spec = &servicebundle.HelmSource{
		Repository: "r", Chart: "c", Version: "v",
		Values: map[string]any{"cluster": map[string]any{"instances": 1}},
	}
	bundle.Renderer.ValueMapping = []servicebundle.ValueMappingItem{
		{ClaimPath: ".spec.parameters.size.replicas", Target: "cluster.instances"},
	}

	files := fstest.MapFS{
		"tmpl.yaml": &fstest.MapFile{Data: []byte(`
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
spec:
  values:
    $slapperRef: renderer.values.kcl
`)},
	}
	entry := StepEntry{
		Kind:           servicebundle.StepProvisioning,
		Function:       FunctionRef{Name: "function-kcl"},
		InputTemplates: map[string]string{"helm": "tmpl.yaml"},
	}
	r := newRenderer(entry, files, bundle)

	out, err := r.Render(servicebundle.PipelineStep{Kind: servicebundle.StepProvisioning}, 0)
	require.NoError(t, err)
	input := out["input"].(map[string]any)
	spec := input["spec"].(map[string]any)
	want := "{\n" +
		"    cluster = {\n" +
		"        instances = oxr.spec.parameters.size.replicas\n" +
		"    }\n" +
		"}"
	assert.Equal(t, want, spec["values"])
}

// silence unused import warning when removing the old testdata-based test
var _ = context.Background
