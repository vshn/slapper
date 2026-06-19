package stdlib

import (
	"fmt"

	"github.com/vshn/slapper/pkg/servicebundle"
)

const (
	ManifestAPIVersion = "slapper.appslap.io/v1alpha1"
	ManifestKind       = "Stdlib"
)

// Manifest describes the stdlib schema.
type Manifest struct {
	APIVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Metadata   ManifestMeta `json:"metadata"`
	// Steps contains a list of steps that the stdlib provides.
	// Multiple steps of the same kind are invalid.
	Steps           []StepEntry       `json:"steps,omitempty"`
	SchemaFragments map[string]string `json:"schemaFragments,omitempty"`
	Plans           *PlansSection     `json:"plans,omitempty"`
}

// ManifestMeta contains the schema for the Manifest metadata
type ManifestMeta struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// StepEntry contains the schema for a single step.
type StepEntry struct {
	Kind           servicebundle.PipelineStepKind `json:"kind"`
	Function       FunctionRef                    `json:"function"`
	InputFile      string                         `json:"inputFile"`
	InputTemplates map[string]string              `json:"inputTemplates"`
}

// FunctionRef schema for the function reference
type FunctionRef struct {
	Name              string `json:"name"`
	VersionConstraint string `json:"versionConstraint"`
}

// PlansSection schema for default plans
// TODO: just a stub and unused for now
type PlansSection struct {
	Schema string `json:"schema"`
}

// Validate validates the following:
// - valid kind
// - valid APIVersion
// - duplicate steps with the same PipelineStepKind
func (m *Manifest) Validate() error {
	if m.Kind != ManifestKind {
		return fmt.Errorf("unsupported stdlib kind: %s, have: %s", ManifestKind, m.Kind)
	}

	if m.APIVersion != ManifestAPIVersion {
		return fmt.Errorf("unsupported stdlib apiVersion want: %s, have: %s", ManifestAPIVersion, m.APIVersion)
	}

	seen := map[servicebundle.PipelineStepKind]bool{}
	for _, step := range m.Steps {
		if seen[step.Kind] {
			return fmt.Errorf("duplicate step kind detected: %s", step.Kind)
		}
		seen[step.Kind] = true

		if step.Function.Name == "" {
			return fmt.Errorf("step %s: function.name is required", step.Kind)
		}

		hasFile := step.InputFile != ""
		hasTpls := len(step.InputTemplates) > 0

		if hasFile == hasTpls {
			return fmt.Errorf("step %s: exactly one of inputFile or inputTemplates must be set", step.Kind)
		}

		if step.Kind == servicebundle.StepProvisioning && hasFile {
			return fmt.Errorf("step %s: must use inputTemplates (not inputFile)", step.Kind)
		}
		if step.Kind != servicebundle.StepProvisioning && hasTpls {
			return fmt.Errorf("step %s: must use inputFile (not inputTemplates)", step.Kind)
		}
	}

	return nil
}
