package stdlib

import (
	"fmt"
	"io/fs"

	"sigs.k8s.io/yaml"

	"github.com/vshn/slapper/pkg/converter/pipeline"
	"github.com/vshn/slapper/pkg/converter/renderer/helm"
	"github.com/vshn/slapper/pkg/converter/template"
	"github.com/vshn/slapper/pkg/servicebundle"
)

type stdlibRenderer struct {
	entry  StepEntry
	files  fs.FS
	bundle *servicebundle.ServiceBundle
}

func newRenderer(entry StepEntry, files fs.FS, sb *servicebundle.ServiceBundle) *stdlibRenderer {
	return &stdlibRenderer{
		entry:  entry,
		files:  files,
		bundle: sb,
	}
}

// Kind returns the kind of the step entry
func (s *stdlibRenderer) Kind() servicebundle.PipelineStepKind {
	return s.entry.Kind
}

// Render renders the step entry. The input file is YAML-decoded into a
// structured map so the resulting composition embeds the function input as
// a nested object (what Crossplane's runtime expects), not a raw string.
func (s *stdlibRenderer) Render(_ servicebundle.PipelineStep, i int) (map[string]any, error) {
	if s.entry.Kind == servicebundle.StepProvisioning {
		return s.renderProvisioning(i)
	}

	return s.renderGeneric(i)
}

func (s *stdlibRenderer) renderGeneric(i int) (map[string]any, error) {
	rawInput, err := fs.ReadFile(s.files, s.entry.InputFile)
	if err != nil {
		return nil, fmt.Errorf("input file: %w", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(rawInput, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.entry.InputFile, err)
	}

	return map[string]any{
		// Steps in the composition need to have unique names.
		// so we just append the index of the step from the servicebundle
		"step":        fmt.Sprintf("%s-%d", s.entry.Kind, i),
		"functionRef": map[string]any{"name": pipeline.DeriveFunctionName(s.entry.Function.Name)},
		"input":       parsed,
	}, nil
}

func (s *stdlibRenderer) renderProvisioning(i int) (map[string]any, error) {
	sb := s.bundle

	path, ok := s.entry.InputTemplates[string(sb.Renderer.Type)]
	if !ok {
		return map[string]any{}, fmt.Errorf("stdlib has no template for renderer type %s", sb.Renderer.Type)
	}

	rawTemplate, err := fs.ReadFile(s.files, path)
	if err != nil {
		return nil, fmt.Errorf("getting renderer template: %w", err)
	}

	tpl := map[string]any{}
	if err := yaml.Unmarshal(rawTemplate, &tpl); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	r := template.New()
	r.Register("renderer", helm.New(sb.Renderer))

	resolved, err := r.Resolve(tpl)
	if err != nil {
		return nil, fmt.Errorf("resolving template %s: %w", path, err)
	}

	return map[string]any{
		"step":        fmt.Sprintf("%s-%d", s.entry.Kind, i),
		"functionRef": map[string]any{"name": pipeline.DeriveFunctionName(s.entry.Function.Name)},
		"input":       resolved,
	}, nil
}
