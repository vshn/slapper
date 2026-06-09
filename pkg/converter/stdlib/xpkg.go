package stdlib

import (
	"fmt"
	"sort"

	"github.com/vshn/slapper/pkg/servicebundle"
)

type Dependency struct {
	Function string
	Version  string
}

func BuildDependencies(m *Manifest, used []servicebundle.PipelineStep) ([]Dependency, error) {
	byKind := map[servicebundle.PipelineStepKind]StepEntry{}

	for _, step := range m.Steps {
		byKind[step.Kind] = step
	}

	result := map[string]string{}

	for _, step := range used {
		var name, constraint string

		switch step.Kind {
		case servicebundle.StepCustom:
			cs, ok := step.Spec.(*servicebundle.CustomStep)
			if !ok {
				return nil, fmt.Errorf("custom step has wrong spec type %T", step.Spec)
			}
			if cs.Function.Name == "" {
				return nil, fmt.Errorf("custom step is missing function.name")
			}
			name = cs.Function.Name
			constraint = cs.Function.VersionConstraint
		// TODO: might need later reworking to allow
		// injecting additional steps from the stdlib
		default:
			entry, ok := byKind[step.Kind]
			if !ok {
				return nil, fmt.Errorf("no stdlib entry for step kind %q",
					step.Kind)
			}
			name = entry.Function.Name
			constraint = entry.Function.VersionConstraint
		}

		if existing, ok := result[name]; ok {
			if existing != constraint {
				return nil, fmt.Errorf("conflicting version constraints for function %q: %q vs %q",
					name, existing, constraint)
			}
			continue
		}

		result[name] = constraint

	}

	names := make([]string, 0, len(result))
	for n := range result {
		names = append(names, n)
	}

	sort.Strings(names)

	deps := make([]Dependency, 0, len(names))
	for _, n := range names {
		deps = append(deps, Dependency{Function: n, Version: result[n]})
	}

	return deps, nil
}
