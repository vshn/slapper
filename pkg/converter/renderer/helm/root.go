package helm

import (
	"fmt"
	"strings"

	"github.com/vshn/slapper/pkg/converter/kcl"
	"github.com/vshn/slapper/pkg/servicebundle"
)

// Root implements template.Root. The resolver strips the leading "renderer" segment,
// so path starts at the field below renderer (e.g. ["spec","repository"]).
type Root struct {
	R *servicebundle.Renderer
}

// New returns a Root bound to the given Renderer.
func New(r *servicebundle.Renderer) *Root { return &Root{R: r} }

// Resolve handles the subpaths listed in the helm-rendering design:
// type, spec.{repository,chart,version}, spec.values, valueMapping,
// values.kcl, spec.{repository,chart,version}.kcl. Anything else errors.
func (r *Root) Resolve(path []string) (any, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	switch path[0] {
	case "type":
		return string(r.R.Type), nil
	case "spec":
		return r.resolveSpec(path[1:])
	case "valueMapping":
		out := make([]any, 0, len(r.R.ValueMapping))
		for _, vm := range r.R.ValueMapping {
			m := map[string]any{
				"claimPath": vm.ClaimPath,
				"target":    vm.Target,
			}
			if vm.Manifest != "" {
				m["manifest"] = vm.Manifest
			}
			out = append(out, m)
		}
		return out, nil
	case "values":
		return r.resolveValuesComputed(path[1:])
	default:
		return nil, fmt.Errorf("unknown helm root path %q", strings.Join(path, "."))
	}
}

func (r *Root) resolveSpec(rest []string) (any, error) {
	if len(rest) == 0 {
		return nil, fmt.Errorf("spec: missing subpath")
	}
	h, ok := r.R.Spec.(*servicebundle.HelmSource)
	if !ok {
		return nil, fmt.Errorf("renderer spec is not helm: %T", r.R.Spec)
	}
	switch rest[0] {
	case "repository":
		return scalarOrKCL(h.Repository, rest[1:])
	case "chart":
		return scalarOrKCL(h.Chart, rest[1:])
	case "version":
		return scalarOrKCL(h.Version, rest[1:])
	case "values":
		if len(rest) != 1 {
			return nil, fmt.Errorf("spec.values does not accept subpath %v", rest[1:])
		}
		if h.Values == nil {
			return map[string]any{}, nil
		}
		return h.Values, nil
	default:
		return nil, fmt.Errorf("unknown spec subpath %q", rest[0])
	}
}

// scalarOrKCL: if rest is empty → return s; if rest == ["kcl"] → return Emit(s); else error.
func scalarOrKCL(s string, rest []string) (any, error) {
	switch len(rest) {
	case 0:
		return s, nil
	case 1:
		if rest[0] == "kcl" {
			return kcl.Emit(s)
		}
	}
	return nil, fmt.Errorf("unsupported tail %v", rest)
}

// resolveValuesComputed emits a computed representation of the values.
// It merges hard coded values with the valueMapping. Which will
// get converted into kcl via pkg/converter/kcl.
func (r *Root) resolveValuesComputed(rest []string) (any, error) {
	if len(rest) != 1 || rest[0] != "kcl" {
		return nil, fmt.Errorf("values requires .kcl subpath, got %v", rest)
	}
	h, ok := r.R.Spec.(*servicebundle.HelmSource)
	if !ok {
		return nil, fmt.Errorf("renderer spec is not helm: %T", r.R.Spec)
	}
	merged, err := MergeValues(*h, r.R.ValueMapping)
	if err != nil {
		return nil, err
	}
	return kcl.Emit(merged)
}
