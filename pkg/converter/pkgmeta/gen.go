package pkgmeta

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vshn/slapper/pkg/converter/stdlib"
)

const (
	configurationAPIVersion = "meta.pkg.crossplane.io/v1"
	configurationKind       = "Configuration"
)

// BuildConfiguration emits a Crossplane Configuration meta resource for the
// given bundle, listing every function dependency under spec.dependsOn.
// Empty/nil deps produce an empty (non-nil) dependsOn list.
func BuildConfiguration(bundleName string, deps []stdlib.Dependency) (*unstructured.Unstructured, error) {
	dependsOn := make([]any, 0, len(deps))
	for _, d := range deps {
		dependsOn = append(dependsOn, map[string]any{
			"function": d.Function,
			"version":  d.Version,
		})
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": configurationAPIVersion,
			"kind":       configurationKind,
			"metadata": map[string]any{
				"name": bundleName,
			},
			"spec": map[string]any{
				"dependsOn": dependsOn,
			},
		},
	}, nil
}
