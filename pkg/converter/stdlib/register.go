package stdlib

import (
	"io/fs"

	"github.com/vshn/slapper/pkg/converter/pipeline"
)

func RegisterAll(m *Manifest, files fs.FS) error {
	for _, step := range m.Steps {
		pipeline.Register(newRenderer(step, files))
	}

	return nil
}
