package xrd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/vshn/slapper/pkg/servicebundle"
)

func minimalSB() *servicebundle.ServiceBundle {
	return &servicebundle.ServiceBundle{
		Meta:  servicebundle.Meta{Name: "pg", Author: "vshn", Version: "0.1.0", Stdlib: "ghcr.io/vshn/stdlib"},
		Claim: &servicebundle.Claim{Kind: "VSHNPostgreSQL"},
	}
}

func parametersProps(t *testing.T, x *unstructured.Unstructured) map[string]any {
	t.Helper()
	spec, ok := x.Object["spec"].(map[string]any)
	require.True(t, ok, "spec missing or wrong type")
	versions, ok := spec["versions"].([]any)
	require.True(t, ok, "versions missing or wrong type")
	require.Len(t, versions, 1)
	v0 := versions[0].(map[string]any)
	schema := v0["schema"].(map[string]any)
	openapi := schema["openAPIV3Schema"].(map[string]any)
	topProps := openapi["properties"].(map[string]any)
	specObj := topProps["spec"].(map[string]any)
	specProps := specObj["properties"].(map[string]any)
	params := specProps["parameters"].(map[string]any)
	return params["properties"].(map[string]any)
}

func TestBuildXRD_Skeleton(t *testing.T) {
	x, err := BuildXRD(minimalSB())
	require.NoError(t, err)

	assert.Equal(t, "apiextensions.crossplane.io/v2", x.GetAPIVersion())
	assert.Equal(t, "CompositeResourceDefinition", x.GetKind())
	assert.Equal(t, "xvshnpostgresqls.appslap.io", x.GetName())

	labels := x.GetLabels()
	assert.Equal(t, "pg", labels["appslap.io/servicebundle"])
	assert.Equal(t, "vshn", labels["appslap.io/maintainer"])
	assert.Equal(t, "servicebundle", labels["appslap.io/managed-by"])

	spec := x.Object["spec"].(map[string]any)
	assert.Equal(t, "Namespaced", spec["scope"])
	assert.Equal(t, "appslap.io", spec["group"])

	names := spec["names"].(map[string]any)
	assert.Equal(t, "XVSHNPostgreSQL", names["kind"])
	assert.Equal(t, "xvshnpostgresqls", names["plural"])
	assert.Equal(t, "xvshnpostgresql", names["singular"])

	versions := spec["versions"].([]any)
	require.Len(t, versions, 1)
	v0 := versions[0].(map[string]any)
	assert.Equal(t, "v1alpha1", v0["name"])
	assert.Equal(t, true, v0["served"])
	assert.Equal(t, true, v0["referenceable"])
}

func TestBuildXRD_NilClaim(t *testing.T) {
	sb := &servicebundle.ServiceBundle{Meta: servicebundle.Meta{Name: "pg"}}
	_, err := BuildXRD(sb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claim.kind is required")
}

func TestBuildXRD_EmptyKind(t *testing.T) {
	sb := &servicebundle.ServiceBundle{Claim: &servicebundle.Claim{}}
	_, err := BuildXRD(sb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claim.kind is required")
}

func TestBuildXRD_FallbackServiceSchemaHasNoPreserveUnknown(t *testing.T) {
	x, err := BuildXRD(minimalSB())
	require.NoError(t, err)

	svc := parametersProps(t, x)["service"].(map[string]any)
	assert.Equal(t, "object", svc["type"])
	_, hasPreserve := svc["x-kubernetes-preserve-unknown-fields"]
	assert.False(t, hasPreserve, "preserve-unknown-fields must be absent in fallback schema")
}

func TestBuildXRD_WithSimpleSchema(t *testing.T) {
	sb := minimalSB()
	sb.Claim.SimpleSchema = map[string]any{
		"name": `string | default="bar"`,
	}
	x, err := BuildXRD(sb)
	require.NoError(t, err)

	svc := parametersProps(t, x)["service"].(map[string]any)
	props, ok := svc["properties"].(map[string]any)
	require.True(t, ok, "service.properties missing: %+v", svc)
	name, ok := props["name"].(map[string]any)
	require.True(t, ok, "name field missing")
	assert.Equal(t, "string", name["type"])
	assert.Equal(t, "bar", name["default"])

	_, hasPreserve := svc["x-kubernetes-preserve-unknown-fields"]
	assert.False(t, hasPreserve, "preserve-unknown-fields must be absent when SimpleSchema is set")
}

func TestBuildXRD_SimpleSchemaPreservesMaintainerDescription(t *testing.T) {
	sb := minimalSB()
	sb.Claim.SimpleSchema = map[string]any{
		"name": `string`,
	}
	x, err := BuildXRD(sb)
	require.NoError(t, err)

	svc := parametersProps(t, x)["service"].(map[string]any)
	desc, ok := svc["description"].(string)
	require.True(t, ok, "service.description missing")
	assert.NotEmpty(t, desc)
}

func TestBuildXRD_FrameworkPropertiesPresent(t *testing.T) {
	x, err := BuildXRD(minimalSB())
	require.NoError(t, err)
	params := parametersProps(t, x)
	for _, k := range []string{"plan", "instances", "maintenance", "service"} {
		_, ok := params[k]
		assert.True(t, ok, "framework field %q missing", k)
	}
}

func statusSchema(t *testing.T, x *unstructured.Unstructured) map[string]any {
	t.Helper()
	spec := x.Object["spec"].(map[string]any)
	versions := spec["versions"].([]any)
	v0 := versions[0].(map[string]any)
	schema := v0["schema"].(map[string]any)
	openapi := schema["openAPIV3Schema"].(map[string]any)
	topProps := openapi["properties"].(map[string]any)
	return topProps["status"].(map[string]any)
}

func TestBuildXRD_StatusDeclaresReady(t *testing.T) {
	x, err := BuildXRD(minimalSB())
	require.NoError(t, err)

	status := statusSchema(t, x)
	assert.Equal(t, "object", status["type"])
	assert.Equal(t, true, status["x-kubernetes-preserve-unknown-fields"],
		"status must allow unknown fields so Crossplane-injected/composition-written fields are accepted")

	props, ok := status["properties"].(map[string]any)
	require.True(t, ok, "status.properties missing")
	ready, ok := props["ready"].(map[string]any)
	require.True(t, ok, "status.ready must be declared so SSA typed-patch accepts it")
	assert.Equal(t, "boolean", ready["type"])
}

func TestBuildXRD_SimpleSchemaError(t *testing.T) {
	sb := minimalSB()
	sb.Claim.SimpleSchema = map[string]any{
		"name": "not-a-real-simpleschema-type",
	}
	_, err := BuildXRD(sb)
	require.Error(t, err)
}

func TestExampleBundle_BuildsXRD(t *testing.T) {
	raw, err := os.ReadFile("../../../examples/servicebundle.yaml")
	require.NoError(t, err, "read example")

	var sb servicebundle.ServiceBundle
	require.NoError(t, yaml.Unmarshal(raw, &sb), "decode example")

	x, err := BuildXRD(&sb)
	require.NoError(t, err)

	out, err := yaml.Marshal(x.Object)
	require.NoError(t, err)

	var rt unstructured.Unstructured
	require.NoError(t, yaml.Unmarshal(out, &rt.Object), "round-trip unmarshal\n%s", out)

	assert.Equal(t, x.GetAPIVersion(), rt.GetAPIVersion())
	assert.Equal(t, x.GetKind(), rt.GetKind())
	assert.Equal(t, x.GetName(), rt.GetName())
	assert.Equal(t, x.GetLabels(), rt.GetLabels())

	params := parametersProps(t, &rt)
	for _, k := range []string{"plan", "instances", "maintenance", "service"} {
		_, ok := params[k]
		assert.True(t, ok, "framework field %q missing after round-trip", k)
	}

	svc := params["service"].(map[string]any)
	props, ok := svc["properties"].(map[string]any)
	require.True(t, ok, "service.properties missing after round-trip; svc=%+v", svc)
	for _, k := range []string{"majorVersion", "database", "user"} {
		_, ok := props[k]
		assert.True(t, ok, "service field %q missing after round-trip", k)
	}
}
