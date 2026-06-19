// Package helm merges the bundle's helm renderer config (static values +
// valueMapping) into a Go map of values, then formats the result as KCL
// dict-literal text via pkg/converter/kcl.
package helm

import (
	"fmt"
	"maps"
	"strings"

	"github.com/vshn/slapper/pkg/converter/kcl"
	"github.com/vshn/slapper/pkg/servicebundle"
)

// MergeValues applies vm to h.Values (each item deep-sets a kcl.Ref at the target path),
// and returns the merged map. The returned map is suitable for kcl.Emit.
// The input map is not modified.
func MergeValues(h servicebundle.HelmSource, vm []servicebundle.ValueMappingItem) (map[string]any, error) {
	result := maps.Clone(h.Values)

	for _, item := range vm {

		target := strings.Trim(item.Target, ".")
		if target == "" {
			return nil, fmt.Errorf("empty target on mapping for claimPath %q", item.ClaimPath)
		}

		ref, err := claimPathToKCL(item.ClaimPath)
		if err != nil {
			return nil, fmt.Errorf("invalid claim path: %s: %w", item.ClaimPath, err)
		}
		result = setPath(result, strings.Split(target, "."), ref)
	}

	if result == nil {
		result = map[string]any{}
	}

	return result, nil
}

func claimPathToKCL(p string) (kcl.Ref, error) {
	if !strings.HasPrefix(p, ".") {
		return kcl.Ref{}, fmt.Errorf("claim path must start with '.': %s", p)
	}
	return kcl.Ref{Expr: "oxr" + p}, nil
}

// setPath shallow-clones each map it descends through, then writes v at
// the leaf. Sub-trees not on the path keep their original backing maps.
func setPath(m map[string]any, path []string, v kcl.Ref) map[string]any {
	out := maps.Clone(m)
	if out == nil {
		out = map[string]any{}
	}
	cur := out
	for _, k := range path[:len(path)-1] {
		child, _ := cur[k].(map[string]any)
		next := maps.Clone(child)
		if next == nil {
			next = map[string]any{}
		}
		cur[k] = next
		cur = next
	}
	cur[path[len(path)-1]] = v
	return out
}
