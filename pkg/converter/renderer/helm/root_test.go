package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/slapper/pkg/servicebundle"
)

func helmBundleRenderer() *servicebundle.Renderer {
	return &servicebundle.Renderer{
		Type: servicebundle.RendererTypeHelm,
		Spec: &servicebundle.HelmSource{
			Repository: "https://charts.cnpg.io/",
			Chart:      "cluster",
			Version:    "0.4.0",
			Values: map[string]any{
				"fullnameOverride": "pg",
				"cluster":          map[string]any{"instances": 1},
			},
		},
		ValueMapping: []servicebundle.ValueMappingItem{
			{ClaimPath: ".spec.parameters.size.replicas", Target: "cluster.instances"},
		},
	}
}

func TestRoot_Type(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"type"})
	require.NoError(t, err)
	assert.Equal(t, "helm", got)
}

func TestRoot_SpecRepository(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"spec", "repository"})
	require.NoError(t, err)
	assert.Equal(t, "https://charts.cnpg.io/", got)
}

func TestRoot_SpecChart(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"spec", "chart"})
	require.NoError(t, err)
	assert.Equal(t, "cluster", got)
}

func TestRoot_SpecVersion(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"spec", "version"})
	require.NoError(t, err)
	assert.Equal(t, "0.4.0", got)
}

func TestRoot_SpecValues(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"spec", "values"})
	require.NoError(t, err)
	m, ok := got.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pg", m["fullnameOverride"])
}

func TestRoot_ValueMapping(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"valueMapping"})
	require.NoError(t, err)
	list, ok := got.([]any)
	require.True(t, ok)
	require.Len(t, list, 1)
	item, ok := list[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, ".spec.parameters.size.replicas", item["claimPath"])
	assert.Equal(t, "cluster.instances", item["target"])
}

func TestRoot_ValueMapping_NilReturnsEmptyList(t *testing.T) {
	r := New(&servicebundle.Renderer{
		Type:         servicebundle.RendererTypeHelm,
		Spec:         &servicebundle.HelmSource{Repository: "r", Chart: "c", Version: "v"},
		ValueMapping: nil,
	})
	got, err := r.Resolve([]string{"valueMapping"})
	require.NoError(t, err)
	assert.Equal(t, []any{}, got)
}

func TestRoot_SpecValues_NilReturnsEmptyMap(t *testing.T) {
	r := New(&servicebundle.Renderer{
		Type: servicebundle.RendererTypeHelm,
		Spec: &servicebundle.HelmSource{Repository: "r", Chart: "c", Version: "v"},
	})
	got, err := r.Resolve([]string{"spec", "values"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, got)
}

func TestRoot_ValuesKCL(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"values", "kcl"})
	require.NoError(t, err)
	s, ok := got.(string)
	require.True(t, ok)
	// alphabetical: cluster before fullnameOverride
	want := "{\n" +
		"    cluster = {\n" +
		"        instances = oxr.spec.parameters.size.replicas\n" +
		"    }\n" +
		"    fullnameOverride = \"pg\"\n" +
		"}"
	assert.Equal(t, want, s)
}

func TestRoot_SpecRepositoryKCL(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"spec", "repository", "kcl"})
	require.NoError(t, err)
	assert.Equal(t, `"https://charts.cnpg.io/"`, got)
}

func TestRoot_SpecChartKCL(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"spec", "chart", "kcl"})
	require.NoError(t, err)
	assert.Equal(t, `"cluster"`, got)
}

func TestRoot_SpecVersionKCL(t *testing.T) {
	r := New(helmBundleRenderer())
	got, err := r.Resolve([]string{"spec", "version", "kcl"})
	require.NoError(t, err)
	assert.Equal(t, `"0.4.0"`, got)
}

func TestRoot_UnknownPathErrors(t *testing.T) {
	r := New(helmBundleRenderer())
	_, err := r.Resolve([]string{"bogus"})
	require.Error(t, err)
}
