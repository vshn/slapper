// Package xrd handles the rendering of the claim and XRD.
package xrd

import (
	"fmt"

	"github.com/vshn/slapper/pkg/converter/xpconst"
	"github.com/vshn/slapper/pkg/servicebundle"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BuildXRD generates a Crossplane v2 namespaced CompositeResourceDefinition.
func BuildXRD(sb *servicebundle.ServiceBundle) (*unstructured.Unstructured, error) {
	if sb.Claim == nil || sb.Claim.Kind == "" {
		return nil, fmt.Errorf("claim.kind is required")
	}
	c := sb.Claim

	xKind := c.XKind()
	xrPlu := c.XPlu()

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
