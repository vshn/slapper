package pipeline

import (
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vshn/slapper/pkg/converter/xpconst"
	"github.com/vshn/slapper/pkg/servicebundle"
)

// BuildComposition creates a Crossplane v2 compositon containing the steps
// defined in the service bundle. The actual steps are specified externally in the
// referenced stdlib.
func BuildComposition(sb *servicebundle.ServiceBundle) (*unstructured.Unstructured, error) {
	if sb.Claim == nil || sb.Claim.Kind == "" {
		return nil, fmt.Errorf("claim.kind is required")
	}

	if len(sb.Pipeline) == 0 {
		return nil, fmt.Errorf("pipeline must have at least one step")
	}

	xKind := sb.Claim.XKind()
	xrPlu := sb.Claim.XPlu()
	steps, err := populateSteps(sb)
	if err != nil {
		return nil, err
	}

	comp := &unstructured.Unstructured{}
	comp.SetAPIVersion("apiextensions.crossplane.io/v1")
	comp.SetKind("Composition")
	comp.SetName(xrPlu + "." + xpconst.Group)
	comp.SetLabels(map[string]string{
		"appslap.io/managed-by":    "servicebundle",
		"appslap.io/servicebundle": sb.Meta.Name,
		"appslap.io/maintainer":    sb.Meta.Author,
	})

	comp.Object["spec"] = map[string]any{
		"compositeTypeRef": map[string]any{
			"apiVersion": xpconst.Group + "/" + xpconst.Version,
			"kind":       xKind,
		},
		"mode":     "Pipeline",
		"pipeline": steps,
	}

	return comp, nil
}

func populateSteps(sb *servicebundle.ServiceBundle) ([]any, error) {
	steps := make([]any, 0, len(sb.Pipeline))

	for i, step := range sb.Pipeline {
		r, ok := Get(step.Kind)

		if !ok {
			slog.Warn("no renderer registered for step kind", "step", i, "kind", step.Kind, "registered", registeredKinds())
			return nil, fmt.Errorf("step %d (%s): no renderer registered for kind %q", i, step.Kind, step.Kind)
		}

		slog.Debug("rendering pipeline step", "step", i, "kind", step.Kind)

		entry, err := r.Render(step)
		if err != nil {
			return nil, fmt.Errorf("step %d (%s): %w", i, step.Kind, err)
		}

		steps = append(steps, entry)

	}

	return steps, nil
}
