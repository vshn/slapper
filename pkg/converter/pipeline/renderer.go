// Package pipeline converts the serviceBundle pipeline stanza into
// Crossplane compositions.
package pipeline

import "github.com/vshn/slapper/pkg/servicebundle"

// StepRenderer converts a bundle pipelineStep into a
// composition step. It uses an external stdlib to achieve
// that.
type StepRenderer interface {
	Kind() servicebundle.PipelineStepKind
	Render(step servicebundle.PipelineStep) (map[string]any, error)
}

var registry = map[servicebundle.PipelineStepKind]StepRenderer{}

// Register dynamically registers steps from an external stdlib.
// Stdlib renderers override any in-tree dummy with the same kind.
func Register(r StepRenderer) {
	registry[r.Kind()] = r
}

func Get(k servicebundle.PipelineStepKind) (StepRenderer, bool) {
	r, ok := registry[k]
	return r, ok
}

// registeredKinds returns all currently registered step kinds. Useful for
// error/warn messages that need to show maintainers what is available.
func registeredKinds() []servicebundle.PipelineStepKind {
	out := make([]servicebundle.PipelineStepKind, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// resetRegistry will clear the registry, only used for testing
func resetRegistry() {
	registry = map[servicebundle.PipelineStepKind]StepRenderer{}
}
