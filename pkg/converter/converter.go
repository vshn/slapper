// Package converter handles the conversion from the service bundle to Crossplane artifacts
package converter

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"sigs.k8s.io/yaml"

	"github.com/vshn/slapper/pkg/converter/pipeline"
	"github.com/vshn/slapper/pkg/converter/pkgmeta"
	"github.com/vshn/slapper/pkg/converter/stdlib"
	"github.com/vshn/slapper/pkg/converter/xrd"
	"github.com/vshn/slapper/pkg/servicebundle"
)

var ErrBundleNotLoaded = fmt.Errorf("the bundle hasn't been loaded")

type ServiceBundleConverter struct {
	serviceBundle *servicebundle.ServiceBundle
	StdlibSource  stdlib.Source
	OutputDir     string
}

// Convert converts the loaded bundle into a Crossplane package
func (s *ServiceBundleConverter) Convert() error {
	if s.OutputDir == "" {
		s.OutputDir = "xpkg"
	}

	if s.serviceBundle == nil {
		return ErrBundleNotLoaded
	}

	var xrdFragments map[string]any
	if s.StdlibSource != nil {
		// TODO: can we get a context from cobra?
		m, files, err := stdlib.Load(context.Background(), s.StdlibSource)
		if err != nil {
			return fmt.Errorf("loading stdlib: %w", err)
		}

		err = stdlib.RegisterAll(m, files)
		if err != nil {
			return fmt.Errorf("registering stdlib: %w", err)
		}

		deps, err := stdlib.BuildDependencies(m, s.serviceBundle.Pipeline)
		if err != nil {
			return fmt.Errorf("parsing dependencies: %w", err)
		}

		metapkg, err := pkgmeta.BuildConfiguration(s.serviceBundle.Meta.Name, deps)
		if err != nil {
			return fmt.Errorf("writing crossplane meta pkg: %w", err)
		}

		err = writeToFile(metapkg.Object, "crossplane", s.OutputDir)
		if err != nil {
			return err
		}

		functions := make([]string, 0, len(deps))
		for _, d := range deps {
			functions = append(functions, d.Function)
		}
		slog.Info("xpkg dependencies emitted", "count", len(deps), "functions", functions)

		xrdFrags, err := decodeFragments(m, files)
		if err != nil {
			return fmt.Errorf("decoding xrd fragments: %w", err)
		}

		xrdFragments = xrdFrags

	}

	slog.Info("rendering XRD")

	xrdObj, err := xrd.BuildXRD(s.serviceBundle)
	if err != nil {
		return fmt.Errorf("rendering XRD failed: %w", err)
	}

	if xrdFragments != nil {
		err := xrd.MergeFrameworkFragments(xrdObj, xrdFragments)
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(xrdFragments))
		for k := range xrdFragments {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		slog.Info("stdlib schema fragments merged", "keys", keys)
	}

	err = writeToFile(xrdObj.Object, "xrd", s.OutputDir)
	if err != nil {
		return err
	}

	slog.Info("rendering composition")

	comp, err := pipeline.BuildComposition(s.serviceBundle)
	if err != nil {
		return fmt.Errorf("rendering Composition failed: %w", err)
	}

	err = writeToFile(comp.Object, "composition", s.OutputDir)
	if err != nil {
		return err
	}

	return nil
}

func writeToFile(rawData map[string]any, filename, outputDir string) error {
	slog.Debug("writing file", "filename", filename)

	data, err := yaml.Marshal(rawData)
	if err != nil {
		return fmt.Errorf("failed to convert %s to yaml: %w", filename, err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output dir %s: %w", outputDir, err)
	}
	path := filepath.Join(outputDir, filename+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}
	slog.Info("wrote output", "path", path, "bytes", len(data))
	return nil
}

// LoadBundle will load the bundle into memory from the given path.
func (s *ServiceBundleConverter) LoadBundle(path string) error {
	slog.Debug("loading bundle", "path", path)

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	sb := &servicebundle.ServiceBundle{}
	if err := yaml.Unmarshal(content, sb); err != nil {
		return fmt.Errorf("cannot decode serviceBundle: %w", err)
	}

	s.serviceBundle = sb

	attrs := []any{
		"name", sb.Meta.Name,
		"version", sb.Meta.Version,
		"pipelineSteps", len(sb.Pipeline),
	}
	if sb.Claim != nil {
		attrs = append(attrs, "claimKind", sb.Claim.Kind)
	}
	if sb.Renderer != nil {
		attrs = append(attrs, "rendererType", string(sb.Renderer.Type))
	}
	slog.Info("loaded bundle", attrs...)

	return nil
}

func decodeFragments(m *stdlib.Manifest, files fs.FS) (map[string]any, error) {
	out := map[string]any{}

	for k, path := range m.SchemaFragments {
		b, err := fs.ReadFile(files, path)
		if err != nil {
			return nil, fmt.Errorf("read fragment %s: %w", path, err)
		}

		var v map[string]any
		if err := yaml.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("parse fragment %s: %w", path, err)
		}

		out[k] = v
	}

	return out, nil
}
