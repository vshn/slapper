package pipeline

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/vshn/slapper/pkg/servicebundle"
)

type fakeRenderer struct {
	kind servicebundle.PipelineStepKind
}

func (f fakeRenderer) Kind() servicebundle.PipelineStepKind { return f.kind }
func (f fakeRenderer) Render(_ servicebundle.PipelineStep) (map[string]any, error) {
	return map[string]any{"step": string(f.kind)}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	defer resetRegistry()
	resetRegistry()

	r := fakeRenderer{kind: "fake"}
	Register(r)

	got, ok := Get("fake")
	require.True(t, ok, "Get returned ok=false after Register")
	assert.Equal(t, servicebundle.PipelineStepKind("fake"), got.Kind())
}

func TestRegistry_GetMissing(t *testing.T) {
	defer resetRegistry()
	resetRegistry()

	_, ok := Get("nope")
	assert.False(t, ok, "Get returned ok=true for unregistered kind")
}

func TestRegistry_RegisterOverwrites(t *testing.T) {
	defer resetRegistry()
	resetRegistry()

	Register(fakeRenderer{kind: "x"})
	replacement := fakeRenderer{kind: "x"}
	Register(replacement)

	got, _ := Get("x")
	assert.Equal(t, replacement, got, "Register should overwrite existing entry of same kind")
}

func TestBuiltinRenderer_Render(t *testing.T) {
	r := dummyRenderer{kind: servicebundle.StepProvisioning}
	got, err := r.Render(servicebundle.PipelineStep{Kind: servicebundle.StepProvisioning})
	require.NoError(t, err)

	assert.Equal(t, "provisioning", got["step"])

	fn, ok := got["functionRef"].(map[string]any)
	require.True(t, ok, "functionRef not a map")
	assert.Equal(t, "function-kcl", fn["name"])

	in, ok := got["input"].(map[string]any)
	require.True(t, ok, "input not a map")
	assert.NotEmpty(t, in, "input should not be empty")
}

func TestBuiltinRenderer_Kind(t *testing.T) {
	r := dummyRenderer{kind: servicebundle.StepNetworking}
	assert.Equal(t, servicebundle.StepNetworking, r.Kind())
}

func TestCustomRenderer_PassThrough(t *testing.T) {
	step := servicebundle.PipelineStep{
		Kind: servicebundle.StepCustom,
		Spec: &servicebundle.CustomStep{
			Function: servicebundle.FuncRef{
				Name:              "ghcr.io/crossplane-contrib/function-python",
				VersionConstraint: "v0.4.0",
			},
			Input: map[string]any{
				"apiVersion": "python.fn.crossplane.io/v1beta1",
				"kind":       "Script",
				"script":     "print(1)",
			},
		},
	}
	got, err := customRenderer{}.Render(step)
	require.NoError(t, err)

	assert.Equal(t, "custom", got["step"])

	fn := got["functionRef"].(map[string]any)
	assert.Equal(t, "function-python", fn["name"])

	in := got["input"].(map[string]any)
	assert.Equal(t, "Script", in["kind"])
	assert.Equal(t, "print(1)", in["script"])
}

func TestCustomRenderer_WrongSpecType(t *testing.T) {
	step := servicebundle.PipelineStep{
		Kind: servicebundle.StepCustom,
		Spec: &servicebundle.ProvisioningStep{},
	}
	_, err := customRenderer{}.Render(step)
	assert.Error(t, err, "expected error for wrong spec type")
}

func TestCustomRenderer_MissingFunction(t *testing.T) {
	step := servicebundle.PipelineStep{
		Kind: servicebundle.StepCustom,
		Spec: &servicebundle.CustomStep{Function: servicebundle.FuncRef{}},
	}
	_, err := customRenderer{}.Render(step)
	assert.Error(t, err, "expected error for empty function.name")
}

func TestDeriveFunctionName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"oci with tag", "ghcr.io/crossplane-contrib/function-python:v0.4.0", "function-python"},
		{"oci no tag", "ghcr.io/foo/fn", "fn"},
		{"oci with digest", "ghcr.io/foo/function-noop@sha256:abc123", "function-noop"},
		{"bare name", "function-noop", "function-noop"},
		{"empty", "", ""},
		{"trailing slash stripped", "ghcr.io/foo/bar/", "bar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, deriveFunctionName(tc.in))
		})
	}
}

func TestInit_RegistersAllKinds(t *testing.T) {
	// Re-run init explicitly because earlier tests may have called resetRegistry.
	registerDefaults()

	kinds := []servicebundle.PipelineStepKind{
		servicebundle.StepProvisioning,
		servicebundle.StepNetworking,
		servicebundle.StepBackup,
		servicebundle.StepMonitoring,
		servicebundle.StepMaintenance,
		servicebundle.StepCustom,
	}
	for _, k := range kinds {
		_, ok := Get(k)
		assert.True(t, ok, "kind %q not registered after init", k)
	}
}

func minimalBundle() *servicebundle.ServiceBundle {
	return &servicebundle.ServiceBundle{
		Meta:  servicebundle.Meta{Name: "pg", Author: "vshn", Version: "0.1.0", Stdlib: "ghcr.io/vshn/stdlib"},
		Claim: &servicebundle.Claim{Kind: "VSHNPostgreSQL"},
		Pipeline: []servicebundle.PipelineStep{
			{Kind: servicebundle.StepProvisioning, Spec: &servicebundle.ProvisioningStep{}},
		},
	}
}

func TestBuildComposition_Skeleton(t *testing.T) {
	registerDefaults()
	c, err := BuildComposition(minimalBundle())
	require.NoError(t, err)

	assert.Equal(t, "apiextensions.crossplane.io/v1", c.GetAPIVersion())
	assert.Equal(t, "Composition", c.GetKind())
	assert.Equal(t, "xvshnpostgresqls.appslap.io", c.GetName())

	spec := c.Object["spec"].(map[string]any)
	assert.Equal(t, "Pipeline", spec["mode"])

	ref := spec["compositeTypeRef"].(map[string]any)
	assert.Equal(t, "appslap.io/v1alpha1", ref["apiVersion"])
	assert.Equal(t, "XVSHNPostgreSQL", ref["kind"])

	labels := c.GetLabels()
	assert.Equal(t, "pg", labels["appslap.io/servicebundle"])
	assert.Equal(t, "vshn", labels["appslap.io/maintainer"])
}

func TestBuildComposition_MissingClaim(t *testing.T) {
	registerDefaults()
	sb := minimalBundle()
	sb.Claim = nil
	_, err := BuildComposition(sb)
	assert.Error(t, err, "expected error for nil claim")
}

func TestBuildComposition_MissingKind(t *testing.T) {
	registerDefaults()
	sb := minimalBundle()
	sb.Claim.Kind = ""
	_, err := BuildComposition(sb)
	assert.Error(t, err, "expected error for empty claim.kind")
}

func TestBuildComposition_EmptyPipeline(t *testing.T) {
	registerDefaults()
	sb := minimalBundle()
	sb.Pipeline = nil
	_, err := BuildComposition(sb)
	assert.Error(t, err, "expected error for empty pipeline")
}

func TestBuildComposition_AllKinds(t *testing.T) {
	registerDefaults()
	sb := minimalBundle()
	sb.Pipeline = []servicebundle.PipelineStep{
		{Kind: servicebundle.StepProvisioning, Spec: &servicebundle.ProvisioningStep{}},
		{Kind: servicebundle.StepNetworking, Spec: &servicebundle.NetworkingStep{}},
		{Kind: servicebundle.StepBackup, Spec: &servicebundle.BackupStep{}},
		{Kind: servicebundle.StepMonitoring, Spec: &servicebundle.MonitoringStep{}},
		{Kind: servicebundle.StepMaintenance, Spec: &servicebundle.MaintenanceStep{}},
		{Kind: servicebundle.StepCustom, Spec: &servicebundle.CustomStep{
			Function: servicebundle.FuncRef{
				Name:              "ghcr.io/foo/function-python",
				VersionConstraint: "v1",
			},
			Input: map[string]any{"k": "v"},
		}},
	}

	c, err := BuildComposition(sb)
	require.NoError(t, err)

	pipe := c.Object["spec"].(map[string]any)["pipeline"].([]any)
	require.Len(t, pipe, 6)

	// dummyRenderer emits "function-kcl" for every built-in kind.
	// customRenderer derives the function name from the OCI ref via deriveFunctionName.
	want := []struct {
		stepName string
		fnName   string
	}{
		{"provisioning", "function-kcl"},
		{"networking", "function-kcl"},
		{"backup", "function-kcl"},
		{"monitoring", "function-kcl"},
		{"maintenance", "function-kcl"},
		{"custom", "function-python"},
	}
	for i, w := range want {
		entry := pipe[i].(map[string]any)
		assert.Equal(t, w.stepName, entry["step"], "pipe[%d].step", i)
		fn := entry["functionRef"].(map[string]any)
		assert.Equal(t, w.fnName, fn["name"], "pipe[%d].functionRef.name", i)
	}
	customInput := pipe[5].(map[string]any)["input"].(map[string]any)
	assert.Equal(t, "v", customInput["k"], "custom input not preserved")
}

func TestBuildComposition_UnknownKind(t *testing.T) {
	defer resetRegistry()
	resetRegistry()
	registerDefaults()
	// Intentionally do NOT register "made-up".

	sb := minimalBundle()
	sb.Pipeline = []servicebundle.PipelineStep{
		{Kind: "made-up", Spec: &servicebundle.ProvisioningStep{}},
	}
	_, err := BuildComposition(sb)
	assert.Error(t, err, "expected error for unknown kind")
}

func TestBuildComposition_RendererErrorWrapped(t *testing.T) {
	registerDefaults()
	sb := minimalBundle()
	sb.Pipeline = []servicebundle.PipelineStep{
		{Kind: servicebundle.StepCustom, Spec: &servicebundle.CustomStep{Function: servicebundle.FuncRef{}}},
	}
	_, err := BuildComposition(sb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 0", "error should mention step index")
	assert.Contains(t, err.Error(), "custom", "error should mention kind")
}

func TestExampleBundle_BuildsComposition(t *testing.T) {
	registerDefaults()
	raw, err := os.ReadFile("../../../examples/servicebundle.yaml")
	require.NoError(t, err, "read example")

	var sb servicebundle.ServiceBundle
	require.NoError(t, yaml.Unmarshal(raw, &sb), "decode example")

	c, err := BuildComposition(&sb)
	require.NoError(t, err)

	out, err := yaml.Marshal(c.Object)
	require.NoError(t, err)

	var rt unstructured.Unstructured
	require.NoError(t, yaml.Unmarshal(out, &rt.Object), "round-trip unmarshal\n%s", out)

	pipe, ok := rt.Object["spec"].(map[string]any)["pipeline"].([]any)
	require.True(t, ok, "pipeline missing in round-trip: %s", out)
	require.Len(t, pipe, len(sb.Pipeline))

	for i, step := range sb.Pipeline {
		entry := pipe[i].(map[string]any)
		assert.Equal(t, string(step.Kind), entry["step"], "pipe[%d].step", i)
	}
}
