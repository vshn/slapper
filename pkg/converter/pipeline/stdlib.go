package pipeline

import (
	"fmt"
	"log/slog"
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

func (b dummyRenderer) Render(_ servicebundle.PipelineStep, i int) (map[string]any, error) {
	return map[string]any{
		"step": fmt.Sprintf("%s-%d", b.kind, i),
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

func (customRenderer) Render(step servicebundle.PipelineStep, i int) (map[string]any, error) {
	c, ok := step.Spec.(*servicebundle.CustomStep)
	if !ok {
		return nil, fmt.Errorf("custom step has wrong spec type %T", step.Spec)
	}

	if c.Function.Name == "" {
		return nil, fmt.Errorf("custom step is missing function.name")
	}

	derived := DeriveFunctionName(c.Function.Name)
	slog.Debug("resolved custom step function",
		"raw", c.Function.Name,
		"derived", derived,
		"versionConstraint", c.Function.VersionConstraint)

	return map[string]any{
		"step": fmt.Sprintf("custom-%d", i),
		"functionRef": map[string]any{
			"name": derived,
		},
		"input": c.Input,
	}, nil
}

// DeriveFunctionName turns an OCI image reference into a best-guess Function
// resource name. Strategy: take the last non-empty path segment, then strip
// any digest (@sha256:...) and tag (:v1.2.3).
// This is used for custom steps, standard functions will get the
// correct refs from the stdlib.
func DeriveFunctionName(ref string) string {
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

// ResetForTest will reset the registry for testing.
// DO NOT use for non-testing purpose.
func ResetForTest() {
	resetRegistry()
}

// RegisterDefaultsForTest registers the default stdlib for testing.
// DO NOT use for non-testing purpose.
func RegisterDefaultsForTest() {
	registerDefaults()
}
