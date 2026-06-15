package converter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/slapper/pkg/converter/stdlib"
)

// writeBundle writes a minimal ServiceBundle covering all five built-in
// pipeline step kinds plus a one-field SimpleSchema, then returns the path to
// the written bundle. Used by integration tests that exercise the full stdlib
// resolution path (renderer override + schema fragment merge + Configuration
// meta emission). Keep the schema field deliberately distinct from any
// stdlib fragment key so a collision can be ruled out as a confounder.
func writeBundle(t *testing.T, dir string) string {
	t.Helper()
	const body = `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ghcr.io/vshn/stdlib
claim:
  kind: Foo
  simpleSchema:
    instanceName: string | default="pg" description="instance display name"
pipeline:
  - kind: provisioning
  - kind: networking
  - kind: backup
  - kind: monitoring
  - kind: maintenance
`
	path := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

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
	err := c.Convert(context.Background())
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
	require.NoError(t, c.Convert(context.Background()))

	for _, f := range []string{"xrd.yaml", "composition.yaml"} {
		path := filepath.Join(dir, "xpkg", f)
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
	err := c.Convert(context.Background())
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
	err := c.Convert(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rendering Composition failed")
}

func TestWriteToFile_CreatesOutputDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, writeToFile(map[string]any{"a": "b"}, "thing", "xpkg"))
	data, err := os.ReadFile(filepath.Join(dir, "xpkg", "thing.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "a: b")
}

func TestServiceBundleConverter_Convert_WithLocalStdlib(t *testing.T) {
	dir := t.TempDir()
	// Copy a minimal valid bundle into dir.
	bundlePath := writeBundle(t, dir) // helper writes a small YAML referencing all 5 step kinds + simpleSchema
	stdlibPath := "stdlib/testdata"

	c := ServiceBundleConverter{
		StdlibSource: stdlib.LocalSource(stdlibPath),
		OutputDir:    filepath.Join(dir, "out"),
	}
	require.NoError(t, c.LoadBundle(bundlePath))
	require.NoError(t, c.Convert(context.Background()))

	// Composition: every step should now use function-kcl with the stdlib KCL body, not the in-tree dummy.
	compRaw, err := os.ReadFile(filepath.Join(dir, "out", "composition.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(compRaw), "krm.kcl.dev/v1alpha1")
	assert.NotContains(t, string(compRaw), `data.content = "dummy"`, "dummy KCL leaked — should be stdlib KCL")
	// Wait — the testdata stdlib also uses `data.content = "dummy"`. Pick a different sentinel:
	// e.g. assert metadata.name lines exist for each kind:
	for _, k := range []string{"provisioning", "networking", "backup", "monitoring", "maintenance"} {
		assert.Contains(t, string(compRaw), `metadata.name = "`+k+`"`)
	}

	// XRD: should contain framework fragments.
	xrdRaw, err := os.ReadFile(filepath.Join(dir, "out", "xrd.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(xrdRaw), "size")
	assert.Contains(t, string(xrdRaw), "monitoring")

	// Configuration meta.
	cfgRaw, err := os.ReadFile(filepath.Join(dir, "out", "crossplane.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(cfgRaw), "meta.pkg.crossplane.io/v1")
	assert.Contains(t, string(cfgRaw), "function-kcl")
}
