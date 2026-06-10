package converter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalBundleYAML = `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ghcr.io/vshn/stdlib
claim:
  kind: Foo
pipeline:
  - kind: provisioning
`

func TestConvert_NoBundleLoaded(t *testing.T) {
	c := ServiceBundleConverter{}
	err := c.Convert()
	require.ErrorIs(t, err, ErrBundleNotLoaded)
}

func TestLoadBundle_FileMissing(t *testing.T) {
	c := ServiceBundleConverter{}
	err := c.LoadBundle(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err), "expected os.IsNotExist, got %v", err)
}

func TestLoadBundle_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("not: : valid"), 0o644))

	c := ServiceBundleConverter{}
	err := c.LoadBundle(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot decode serviceBundle")
}

func TestLoadBundle_Minimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(path, []byte(minimalBundleYAML), 0o644))

	c := ServiceBundleConverter{}
	require.NoError(t, c.LoadBundle(path))
	require.NotNil(t, c.serviceBundle)
	assert.Equal(t, "pg", c.serviceBundle.Meta.Name)
	assert.Equal(t, "Foo", c.serviceBundle.Claim.Kind)
}

func TestConvert_E2E_CreatesOutputDirAndFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	bundlePath := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(bundlePath, []byte(minimalBundleYAML), 0o644))

	c := ServiceBundleConverter{}
	require.NoError(t, c.LoadBundle(bundlePath))
	require.NoError(t, c.Convert())

	for _, f := range []string{"xrd.yaml", "composition.yaml"} {
		path := filepath.Join(dir, outputDir, f)
		info, err := os.Stat(path)
		require.NoError(t, err, "expected %s", path)
		assert.Greater(t, info.Size(), int64(0), "expected non-empty %s", path)
	}
}

func TestConvert_BuildXRDError(t *testing.T) {
	// Missing claim.kind triggers BuildXRD error path.
	bundle := `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ghcr.io/vshn/stdlib
claim:
  kind: ""
pipeline:
  - kind: provisioning
`
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(path, []byte(bundle), 0o644))

	c := ServiceBundleConverter{}
	require.NoError(t, c.LoadBundle(path))
	err := c.Convert()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rendering XRD failed")
}

func TestConvert_BuildCompositionError(t *testing.T) {
	// Empty pipeline triggers BuildComposition error.
	bundle := `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ghcr.io/vshn/stdlib
claim:
  kind: Foo
`
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(path, []byte(bundle), 0o644))

	c := ServiceBundleConverter{}
	require.NoError(t, c.LoadBundle(path))
	err := c.Convert()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rendering Composition failed")
}

func TestWriteToFile_CreatesOutputDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, writeToFile(map[string]any{"a": "b"}, "thing"))
	data, err := os.ReadFile(filepath.Join(dir, outputDir, "thing.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "a: b")
}
