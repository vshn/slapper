package template

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRoot is a test Root that walks a static map[string]any.
type fakeRoot struct {
	data map[string]any
}

func (f fakeRoot) Resolve(path []string) (any, error) {
	cur := any(f.data)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path segment %q on non-map", p)
		}
		v, ok := m[p]
		if !ok {
			return nil, fmt.Errorf("missing %q", p)
		}
		cur = v
	}
	return cur, nil
}

func newResolverWith(name string, data map[string]any) *Resolver {
	r := New()
	r.Register(name, fakeRoot{data: data})
	return r
}

func TestResolver_ReplacesScalarPlaceholder(t *testing.T) {
	r := newResolverWith("root", map[string]any{"x": "hello"})
	tree := map[string]any{"k": map[string]any{"$slapperRef": "root.x"}}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": "hello"}, out)
}

func TestResolver_ReplacesMapPlaceholder(t *testing.T) {
	r := newResolverWith("root", map[string]any{"x": map[string]any{"a": 1}})
	tree := map[string]any{"k": map[string]any{"$slapperRef": "root.x"}}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": map[string]any{"a": 1}}, out)
}

func TestResolver_ReplacesInsideList(t *testing.T) {
	r := newResolverWith("root", map[string]any{"x": "v"})
	tree := map[string]any{
		"items": []any{map[string]any{"$slapperRef": "root.x"}, "static"},
	}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"items": []any{"v", "static"}}, out)
}

func TestResolver_RegularSingleKeyMapUntouched(t *testing.T) {
	r := newResolverWith("root", map[string]any{})
	tree := map[string]any{"normalKey": "value"}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"normalKey": "value"}, out)
}

func TestResolver_NestedPlaceholder(t *testing.T) {
	r := newResolverWith("root", map[string]any{"x": "leaf"})
	tree := map[string]any{
		"outer": map[string]any{
			"inner": map[string]any{"$slapperRef": "root.x"},
		},
	}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"outer": map[string]any{"inner": "leaf"}}, out)
}

func TestResolver_UnknownRootErrors(t *testing.T) {
	r := New()
	tree := map[string]any{"k": map[string]any{"$slapperRef": "nope.x"}}
	_, err := r.Resolve(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func TestResolver_RootErrorPropagates(t *testing.T) {
	r := newResolverWith("root", map[string]any{})
	tree := map[string]any{"k": map[string]any{"$slapperRef": "root.missing"}}
	_, err := r.Resolve(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestResolver_NoRecursionIntoResolvedString(t *testing.T) {
	// resolved value contains text that LOOKS like a placeholder. Must not be re-walked.
	r := newResolverWith("root", map[string]any{"x": `{$slapperRef: nope.x}`})
	tree := map[string]any{"k": map[string]any{"$slapperRef": "root.x"}}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": `{$slapperRef: nope.x}`}, out)
}

func TestResolver_NoRecursionIntoResolvedMap(t *testing.T) {
	// resolved value is a map containing what looks like another placeholder.
	// Resolver must not walk into it.
	inner := map[string]any{"$slapperRef": "nope.x"}
	r := newResolverWith("root", map[string]any{"x": inner})
	tree := map[string]any{"k": map[string]any{"$slapperRef": "root.x"}}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": inner}, out)
}

func TestResolver_PlaceholderValueMustBeString(t *testing.T) {
	r := New()
	tree := map[string]any{"k": map[string]any{"$slapperRef": 42}}
	_, err := r.Resolve(tree)
	require.Error(t, err)
}

func TestResolver_EmptyPathErrors(t *testing.T) {
	r := New()
	tree := map[string]any{"k": map[string]any{"$slapperRef": ""}}
	_, err := r.Resolve(tree)
	require.Error(t, err)
}

func TestResolver_StringWithSingleRef(t *testing.T) {
	r := newResolverWith("root", map[string]any{"x": "hello"})
	tree := map[string]any{"k": "foo {$slapperRef: root.x} bar"}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": "foo hello bar"}, out)
}

func TestResolver_StringWithMultipleRefs(t *testing.T) {
	r := newResolverWith("root", map[string]any{"a": "x", "b": "y"})
	tree := map[string]any{"k": "{$slapperRef: root.a}/{$slapperRef: root.b}"}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": "x/y"}, out)
}

func TestResolver_StringWithoutRefUnchanged(t *testing.T) {
	r := newResolverWith("root", map[string]any{"x": "v"})
	tree := map[string]any{"k": "no refs here"}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": "no refs here"}, out)
}

func TestResolver_StringRefInsideBlockScalar(t *testing.T) {
	r := newResolverWith("renderer", map[string]any{
		"spec": map[string]any{
			"repository": map[string]any{"kcl": `"https://charts.cnpg.io/"`},
			"chart":      map[string]any{"kcl": `"cluster"`},
		},
	})
	src := `release = {
  chart = {
    repository = {$slapperRef: renderer.spec.repository.kcl}
    name       = {$slapperRef: renderer.spec.chart.kcl}
  }
}`
	tree := map[string]any{"spec": map[string]any{"source": src}}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	spec := out["spec"].(map[string]any)
	resolved := spec["source"].(string)
	assert.Contains(t, resolved, `repository = "https://charts.cnpg.io/"`)
	assert.Contains(t, resolved, `name       = "cluster"`)
	assert.NotContains(t, resolved, "$slapperRef")
}

func TestResolver_StringRefWhitespaceTolerant(t *testing.T) {
	r := newResolverWith("root", map[string]any{"x": "v"})
	tree := map[string]any{"k": "{  $slapperRef:  root.x  }"}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": "v"}, out)
}

func TestResolver_StringRefUnknownRootErrors(t *testing.T) {
	r := New()
	tree := map[string]any{"k": "prefix {$slapperRef: nope.x} suffix"}
	_, err := r.Resolve(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func TestResolver_StringRefRootErrorPropagates(t *testing.T) {
	r := newResolverWith("root", map[string]any{})
	tree := map[string]any{"k": "x {$slapperRef: root.missing} y"}
	_, err := r.Resolve(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestResolver_StringRefResolvedValueNotReResolved(t *testing.T) {
	// Resolved fragment contains text that looks like another ref. Must NOT
	// be re-processed (single-pass guarantee).
	r := newResolverWith("root", map[string]any{"x": "{$slapperRef: nope.y}"})
	tree := map[string]any{"k": "before {$slapperRef: root.x} after"}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": "before {$slapperRef: nope.y} after"}, out)
}

func TestResolver_StringRefNonStringValueFormatted(t *testing.T) {
	// Resolver formats non-string scalars via %v.
	r := newResolverWith("root", map[string]any{"n": 42})
	tree := map[string]any{"k": "count={$slapperRef: root.n}"}
	out, err := r.Resolve(tree)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": "count=42"}, out)
}
