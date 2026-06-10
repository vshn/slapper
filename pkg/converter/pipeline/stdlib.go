package pipeline

import (
	"fmt"
	"strings"

	"github.com/vshn/slapper/pkg/servicebundle"
)

func init() {
	registerDefaults()
}

func registerDefaults() {
	// TODO: these can be removed one by one
	// with actual stdlib functions
	for _, k := range []servicebundle.PipelineStepKind{
		servicebundle.StepProvisioning,
		servicebundle.StepNetworking,
		servicebundle.StepBackup,
		servicebundle.StepMonitoring,
		servicebundle.StepMaintenance,
	} {
		Register(dummyRenderer{kind: k})
	}
	Register(customRenderer{})
}

// dummyRenderer is a dummy renderer which only produces kcl functions
// which create configmaps with the step kind as the name.
// Only used for debugging and development.
type dummyRenderer struct {
	kind servicebundle.PipelineStepKind
}

func (b dummyRenderer) Kind() servicebundle.PipelineStepKind { return b.kind }

func (b dummyRenderer) Render(_ servicebundle.PipelineStep) (map[string]any, error) {
	return map[string]any{
		"step": string(b.kind),
		"functionRef": map[string]any{
			"name": "function-kcl",
		},
		"input": map[string]any{
			"apiVersion": "krm.kcl.dev/v1alpha1",
			"kind":       "KCLInput",
			"spec": map[string]any{
				"source": fmt.Sprintf(`
# Read the XR
oxr = option("params").oxr
# Construct a cm
cm = {
    apiVersion = "v1"
    kind = "ConfigMap"
    metadata.name = %q
    metadata.annotations: {
    	"krm.kcl.dev/ready": "True"
    }
    data.content = "dummy"
}
# Return the cm
items = [cm]
					`, string(b.kind)),
			},
		},
	}, nil
}

// customRenderer takes a custom step and generates a
// composition step out of it.
type customRenderer struct{}

func (customRenderer) Kind() servicebundle.PipelineStepKind { return servicebundle.StepCustom }

func (customRenderer) Render(step servicebundle.PipelineStep) (map[string]any, error) {
	c, ok := step.Spec.(*servicebundle.CustomStep)
	if !ok {
		return nil, fmt.Errorf("custom step has wrong spec type %T", step.Spec)
	}

	if c.Function.Name == "" {
		return nil, fmt.Errorf("custom step is missing function.name")
	}

	return map[string]any{
		"step": "custom",
		"functionRef": map[string]any{
			"name": deriveFunctionName(c.Function.Name),
		},
		"input": c.Input,
	}, nil
}

// deriveFunctionName turns an OCI image reference into a best-guess Function
// resource name. Strategy: take the last non-empty path segment, then strip
// any digest (@sha256:...) and tag (:v1.2.3).
// This is used for custom steps, standard functions will get the
// correct refs from the stdlib.
func deriveFunctionName(ref string) string {
	ref = strings.TrimRight(ref, "/")
	if ref == "" {
		return ""
	}
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.Index(ref, ":"); i >= 0 {
		ref = ref[:i]
	}
	return ref
}
