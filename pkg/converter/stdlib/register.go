package stdlib

import (
	"io/fs"
	"log/slog"

	"github.com/vshn/slapper/pkg/converter/pipeline"
	"github.com/vshn/slapper/pkg/servicebundle"
)

func RegisterAll(m *Manifest, files fs.FS) error {
	kinds := make([]servicebundle.PipelineStepKind, 0, len(m.Steps))
	for _, step := range m.Steps {
		pipeline.Register(newRenderer(step, files))
		kinds = append(kinds, step.Kind)
	}

	slog.Info("stdlib steps registered", "kinds", kinds)
	return nil
}
