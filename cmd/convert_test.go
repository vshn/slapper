package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeBundleWithStdlibRef writes a minimal-but-convertible ServiceBundle
// whose meta.stdlib points at the given OCI ref, then returns the bundle
// path. Used by --no-stdlib warn-path tests: the bundle declares a stdlib
// reference, the CLI flag suppresses resolution, conversion must still
// succeed via the in-tree dummy renderers.
func writeBundleWithStdlibRef(t *testing.T, dir, ref string) string {
	t.Helper()
	body := fmt.Sprintf(`
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: %s
claim:
  kind: Foo
renderer:
  type: helm
  repository: https://charts.cnpg.io/
  chart: cluster
  version: 0.4.0
pipeline:
  - kind: provisioning
`, ref)
	path := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

// minimalBundle omits meta.stdlib so the CLI's stdlib-resolution
// short-circuits (no flag + no meta.stdlib → nil source). Tests that need
// stdlib behavior set the field explicitly via writeBundleWithStdlibRef.
const minimalBundle = `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ""
claim:
  kind: Foo
renderer:
  type: helm
  repository: https://charts.cnpg.io/
  chart: cluster
  version: 0.4.0
pipeline:
  - kind: provisioning
`

const helmBundle = `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ""
claim:
  kind: Foo
renderer:
  type: helm
  repository: https://charts.cnpg.io/
  chart: cluster
  version: 0.4.0
pipeline:
  - kind: provisioning
`

const helmBundleNoProvisioning = `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ""
claim:
  kind: Foo
renderer:
  type: helm
  repository: https://charts.cnpg.io/
  chart: cluster
  version: 0.4.0
pipeline:
  - kind: backup
`

const helmBundleNoRenderer = `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ""
claim:
  kind: Foo
pipeline:
  - kind: provisioning
`

const helmBundleInvalidRenderer = `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ""
claim:
  kind: Foo
renderer:
  type: helm
  repository: https://charts.cnpg.io/
pipeline:
  - kind: provisioning
`

func TestConvert_NoArgs(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"convert"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no file path provided")
}

func TestConvert_LoadFailure(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"convert", filepath.Join(t.TempDir(), "missing.yaml")})
	err := root.Execute()
	require.Error(t, err)
}

func TestConvert_E2E(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	bundlePath := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(bundlePath, []byte(minimalBundle), 0o644))

	root := newRootCmd()
	root.SetArgs([]string{"convert", bundlePath})
	require.NoError(t, root.Execute())

	for _, f := range []string{"xrd.yaml", "composition.yaml"} {
		_, err := os.Stat(filepath.Join(dir, "xpkg", f))
		require.NoError(t, err, "expected %s", f)
	}
}

func TestConvert_FlagsMutuallyExclusive(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"convert", "--stdlib-path", "/tmp/x", "--no-stdlib", "bundle.yaml"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestConvert_StdlibPath_NonexistentDir(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"convert", "--stdlib-path", "/does/not/exist", "bundle.yaml"})
	err := root.Execute()
	require.Error(t, err)
}

func TestConvert_MissingRendererErrors(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(bundlePath, []byte(helmBundleNoRenderer), 0o644))

	root := newRootCmd()
	root.SetArgs([]string{"convert", "--no-stdlib", "--output", filepath.Join(dir, "out"), bundlePath})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "renderer")
}

func TestConvert_MissingProvisioningErrors(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(bundlePath, []byte(helmBundleNoProvisioning), 0o644))

	root := newRootCmd()
	root.SetArgs([]string{"convert", "--no-stdlib", "--output", filepath.Join(dir, "out"), bundlePath})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provisioning")
}

func TestConvert_InvalidRendererSpecErrors(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(bundlePath, []byte(helmBundleInvalidRenderer), 0o644))

	root := newRootCmd()
	root.SetArgs([]string{"convert", "--no-stdlib", "--output", filepath.Join(dir, "out"), bundlePath})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chart")
}

func TestConvert_ValidBundleSucceeds(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(bundlePath, []byte(helmBundle), 0o644))

	root := newRootCmd()
	root.SetArgs([]string{"convert", "--no-stdlib", "--output", filepath.Join(dir, "out"), bundlePath})
	require.NoError(t, root.Execute())
}

func TestConvert_NoStdlibWithMetaSet_Warn(t *testing.T) {
	// Write minimal bundle with Meta.Stdlib set; run with --no-stdlib.
	// Assert exit succeeds (warn only). Compositions emitted with in-tree dummies.
	dir := t.TempDir()
	bundlePath := writeBundleWithStdlibRef(t, dir, "ghcr.io/fake/stdlib:v0")
	out := filepath.Join(dir, "out")

	root := newRootCmd()
	root.SetArgs([]string{"convert", "--no-stdlib", "--output", out, bundlePath})
	require.NoError(t, root.Execute())

	_, err := os.Stat(filepath.Join(out, "composition.yaml"))
	require.NoError(t, err)
}
