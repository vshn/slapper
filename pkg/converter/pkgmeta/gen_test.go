package pkgmeta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/slapper/pkg/converter/stdlib"
)

func TestBuildConfiguration_BasicShape(t *testing.T) {
	deps := []stdlib.Dependency{
		{Function: "xpkg.upbound.io/crossplane-contrib/function-kcl", Version: ">=v0.10.0"},
	}
	cfg, err := BuildConfiguration("postgresql", deps)
	require.NoError(t, err)

	assert.Equal(t, "meta.pkg.crossplane.io/v1", cfg.GetAPIVersion())
	assert.Equal(t, "Configuration", cfg.GetKind())
	assert.Equal(t, "postgresql", cfg.GetName())

	spec, ok := cfg.Object["spec"].(map[string]any)
	require.True(t, ok)
	dependsOn, ok := spec["dependsOn"].([]any)
	require.True(t, ok)
	require.Len(t, dependsOn, 1)
	d := dependsOn[0].(map[string]any)
	assert.Equal(t, "xpkg.upbound.io/crossplane-contrib/function-kcl", d["function"])
	assert.Equal(t, ">=v0.10.0", d["version"])
}

func TestBuildConfiguration_EmptyDeps(t *testing.T) {
	cfg, err := BuildConfiguration("svc", nil)
	require.NoError(t, err)
	spec := cfg.Object["spec"].(map[string]any)
	dependsOn, _ := spec["dependsOn"].([]any)
	assert.Empty(t, dependsOn)
}
