// Package xrd handles the rendering of the claim and XRD.
package xrd

import (
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vshn/slapper/pkg/converter/xpconst"
	"github.com/vshn/slapper/pkg/servicebundle"
)

// BuildXRD generates a Crossplane v2 namespaced CompositeResourceDefinition.
func BuildXRD(sb *servicebundle.ServiceBundle) (*unstructured.Unstructured, error) {
	if sb.Claim == nil || sb.Claim.Kind == "" {
		return nil, fmt.Errorf("claim.kind is required")
	}
	c := sb.Claim

	xKind := c.XKind()
	xrPlu := c.XPlu()
	slog.Debug("derived composite names", "claimKind", c.Kind, "xKind", xKind, "xPlural", xrPlu, "xSingular", c.XSing())

	params, err := buildParameterSchema(sb)
	if err != nil {
		return nil, err
	}

	xrd := &unstructured.Unstructured{}
	xrd.SetAPIVersion("apiextensions.crossplane.io/v2")
	xrd.SetKind("CompositeResourceDefinition")
	xrd.SetName(xrPlu + "." + xpconst.Group)
	xrd.SetLabels(map[string]string{
		"appslap.io/managed-by":    "servicebundle",
		"appslap.io/servicebundle": sb.Meta.Name,
		"appslap.io/maintainer":    sb.Meta.Author,
	})

	// we're using unstructured.unstructured, because
	// Crossplane's upstream XRD struct also just
	// contains a raw map[string]any for openAPIV3Schema
	xrd.Object["spec"] = map[string]any{
		"scope": "Namespaced",
		"group": xpconst.Group,
		"names": map[string]any{
			"kind":       xKind,
			"plural":     xrPlu,
			"singular":   c.XSing(),
			"shortNames": c.ShortNames,
		},
		"versions": []any{
			map[string]any{
				"name":          xpconst.Version,
				"served":        true,
				"referenceable": true,
				"schema": map[string]any{
					"openAPIV3Schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"spec": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"parameters": params,
								},
							},
							"status": map[string]any{
								"type":                                 "object",
								"x-kubernetes-preserve-unknown-fields": true,
								"properties": map[string]any{
									// Composition functions write status.ready to signal
									// the instance is reconciled. Crossplane does not
									// auto-inject this field, and SSA's typed-patch path
									// rejects it unless explicitly declared.
									"ready": map[string]any{
										"type":        "boolean",
										"description": "Ready indicates whether the composite resource has been fully reconciled and is ready for use.",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return xrd, nil
}

func MergeFrameworkFragments(xrd *unstructured.Unstructured, fragments map[string]any) error {
	if fragments == nil {
		return nil
	}

	raw, _, err := unstructured.NestedFieldNoCopy(xrd.Object, "spec", "versions")
	if err != nil {
		return fmt.Errorf("xrd cannot get versions: %w", err)
	}

	versions, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("xrd versions has unexpected type %T", raw)
	}
	if len(versions) == 0 {
		return fmt.Errorf("xrd has no versions")
	}

	// Crossplane doesn't really support multiple versions at the moment...
	v0, ok := versions[0].(map[string]any)
	if !ok {
		return fmt.Errorf("xrd version[0] has unexpected type %T", versions[0])
	}

	raw, found, err := unstructured.NestedFieldNoCopy(v0,
		"schema", "openAPIV3Schema",
		"properties", "spec",
		"properties", "parameters", "properties")
	if err != nil {
		return fmt.Errorf("xrd parameters properties: %w", err)
	}

	var props map[string]any
	if !found {
		props = map[string]any{}
	} else {
		props, ok = raw.(map[string]any)
		if !ok {
			return fmt.Errorf("xrd parameters properties has unexpected type %T", raw)
		}
	}

	for k, v := range fragments {
		if _, exists := props[k]; exists {
			return fmt.Errorf("service schema field %s collides with framework fragment", k)
		}
		props[k] = v
	}

	// NestedFieldNoCopy returns the in-tree map on the found path, so mutations
	// already propagated. Only need an explicit Set when we created props fresh.
	if !found {
		return unstructured.SetNestedField(v0, props,
			"schema", "openAPIV3Schema",
			"properties", "spec",
			"properties", "parameters", "properties")
	}
	return nil
}
