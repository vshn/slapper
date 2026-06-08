package stdlib

import (
	"fmt"
	"io/fs"
	"os"

	"sigs.k8s.io/yaml"
)

func loadLocal(dir string) (*Manifest, fs.FS, error) {
	_, err := os.Stat(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("error opening %s: %w", dir, err)
	}

	fsys := os.DirFS(dir)

	raw, err := fs.ReadFile(fsys, manifestFile)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read %s: %w", manifestFile, err)
	}

	m := &Manifest{}
	err = yaml.Unmarshal(raw, m)
	if err != nil {
		return nil, nil, fmt.Errorf("error decoding %s: %w", manifestFile, err)
	}

	if err := m.Validate(); err != nil {
		return nil, nil, err
	}

	for _, s := range m.Steps {
		_, err := fs.Stat(fsys, s.InputFile)
		if err != nil {
			return nil, nil, fmt.Errorf("step %s inputFile %s: %w", s.Kind, s.InputFile, err)
		}
	}

	for k, path := range m.SchemaFragments {
		if _, err := fs.Stat(fsys, path); err != nil {
			return nil, nil, fmt.Errorf("schemaFragment %s path %s: %w", k, path, err)
		}
	}

	return m, fsys, nil
}
