// Package converter handles the conversion from the service bundle to Crossplane artifacts
package converter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vshn/slapper/pkg/converter/pipeline"
	"github.com/vshn/slapper/pkg/converter/xrd"
	"github.com/vshn/slapper/pkg/servicebundle"
	"sigs.k8s.io/yaml"
)

const outputDir = "xpkg"

var ErrBundleNotLoaded = fmt.Errorf("the bundle hasn't been loaded")

type ServiceBundleConverter struct {
	serviceBundle *servicebundle.ServiceBundle
}

// Convert converts the loaded bundle into a Crossplane package
func (s *ServiceBundleConverter) Convert() error {
	if s.serviceBundle == nil {
		return ErrBundleNotLoaded
	}

	xrd, err := xrd.BuildXRD(s.serviceBundle)
	if err != nil {
		return fmt.Errorf("rendering XRD failed: %w", err)
	}

	err = writeToFile(xrd.Object, "xrd")
	if err != nil {
		return err
	}

	comp, err := pipeline.BuildComposition(s.serviceBundle)
	if err != nil {
		return fmt.Errorf("rendering Composition failed: %w", err)
	}

	err = writeToFile(comp.Object, "composition")
	if err != nil {
		return err
	}

	return nil
}

func writeToFile(rawData map[string]any, filename string) error {
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
	return nil
}

// LoadBundle will load the bundle into memory from the given path.
func (s *ServiceBundleConverter) LoadBundle(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	sb := &servicebundle.ServiceBundle{}
	if err := yaml.Unmarshal(content, sb); err != nil {
		return fmt.Errorf("cannot decode serviceBundle: %w", err)
	}

	s.serviceBundle = sb

	return nil
}
