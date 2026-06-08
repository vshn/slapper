package stdlib

import (
	"fmt"
	"io/fs"

	"github.com/vshn/slapper/pkg/converter/pipeline"
	"github.com/vshn/slapper/pkg/servicebundle"
)

type stdlibRenderer struct {
	entry StepEntry
	files fs.FS
}

func newRenderer(entry StepEntry, files fs.FS) *stdlibRenderer {
	return &stdlibRenderer{
		entry: entry,
		files: files,
	}
}

// Kind returns the kind of the step entry
func (s *stdlibRenderer) Kind() servicebundle.PipelineStepKind {
	return s.entry.Kind
}

// Render renders the step entry
func (s *stdlibRenderer) Render(_ servicebundle.PipelineStep) (map[string]any, error) {
	rawInput, err := fs.ReadFile(s.files, s.entry.InputFile)
	if err != nil {
		return nil, fmt.Errorf("input file: %w", err)
	}

	return map[string]any{
		"step":        string(s.entry.Kind),
		"functionRef": map[string]any{"name": pipeline.DeriveFunctionName(s.entry.Function.Name)},
		"input":       string(rawInput),
	}, nil
}
