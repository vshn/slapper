package stdlib

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalSource_Load_OK(t *testing.T) {
	src := LocalSource("testdata")
	m, files, err := Load(context.Background(), src)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "test-stdlib", m.Metadata.Name)
	assert.Len(t, m.Steps, 5)
	assert.NotNil(t, files)
}

func TestLocalSource_Load_MissingDir(t *testing.T) {
	src := LocalSource("does-not-exist")
	_, _, err := Load(context.Background(), src)
	require.Error(t, err)
}

func TestLocalSource_Load_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	src := LocalSource(dir)
	_, _, err := Load(context.Background(), src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stdlib.yaml")
}

func TestLocalSource_Load_MissingInputFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stdlib.yaml"), []byte(`
apiVersion: slapper.appslap.io/v1alpha1
kind: Stdlib
metadata: {name: x, version: 0.0.1}
steps:
  - kind: networking
    function: {name: function-kcl, versionConstraint: ">=v0.10"}
    inputFile: templates/missing.kcl
`), 0o644))
	_, _, err := Load(context.Background(), LocalSource(dir))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "templates/missing.kcl")
}

func TestLocalSource_Load_MissingSchemaFragment(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stdlib.yaml"), []byte(`
apiVersion: slapper.appslap.io/v1alpha1
kind: Stdlib
metadata: {name: x, version: 0.0.1}
steps: []
schemaFragments: {size: schemas/missing.yaml}
`), 0o644))
	_, _, err := Load(context.Background(), LocalSource(dir))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schemas/missing.yaml")
}

func TestLoadLocal_MissingInputTemplatePath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stdlib.yaml"), []byte(`
apiVersion: slapper.appslap.io/v1alpha1
kind: Stdlib
metadata: {name: t, version: 0.0.1}
steps:
  - kind: provisioning
    function: {name: f}
    inputTemplates:
      helm: templates/missing.yaml
`), 0o644))

	_, _, err := loadLocal(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inputTemplate")
	assert.Contains(t, err.Error(), "helm")
}

func TestLoadLocal_InputTemplatePathPresent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "helm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stdlib.yaml"), []byte(`
apiVersion: slapper.appslap.io/v1alpha1
kind: Stdlib
metadata: {name: t, version: 0.0.1}
steps:
  - kind: provisioning
    function: {name: f}
    inputTemplates:
      helm: templates/helm.yaml
`), 0o644))

	_, _, err := loadLocal(dir)
	require.NoError(t, err)
}
