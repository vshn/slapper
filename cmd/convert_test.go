package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalBundle = `
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

func TestConvert_NoArgs(t *testing.T) {
	err := convert(convertCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no file path provided")
}

func TestConvert_LoadFailure(t *testing.T) {
	err := convert(convertCmd, []string{filepath.Join(t.TempDir(), "missing.yaml")})
	require.Error(t, err)
}

func TestConvert_E2E(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	bundlePath := filepath.Join(dir, "bundle.yaml")
	require.NoError(t, os.WriteFile(bundlePath, []byte(minimalBundle), 0o644))

	require.NoError(t, convert(convertCmd, []string{bundlePath}))

	for _, f := range []string{"xrd.yaml", "composition.yaml"} {
		_, err := os.Stat(filepath.Join(dir, "xpkg", f))
		require.NoError(t, err, "expected %s", f)
	}
}
