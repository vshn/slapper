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

// Register dynamically registers steps from an external
// stdlib.
// TODO: not yet in use, a stdlib dummy will follow.
func Register(r StepRenderer) {
	registry[r.Kind()] = r
}

func Get(k servicebundle.PipelineStepKind) (StepRenderer, bool) {
	r, ok := registry[k]
	return r, ok
}

// resetRegistry will clear the registry, only used for testing
func resetRegistry() {
	registry = map[servicebundle.PipelineStepKind]StepRenderer{}
}
