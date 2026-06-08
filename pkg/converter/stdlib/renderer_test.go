package stdlib

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/slapper/pkg/servicebundle"
)

func TestStdlibRenderer_Render(t *testing.T) {
	m, files, err := Load(context.Background(), LocalSource("testdata"))
	require.NoError(t, err)

	var entry StepEntry
	for _, s := range m.Steps {
		if s.Kind == servicebundle.StepProvisioning {
			entry = s
			break
		}
	}
	r := newRenderer(entry, files)

	out, err := r.Render(servicebundle.PipelineStep{Kind: servicebundle.StepProvisioning})
	require.NoError(t, err)

	assert.Equal(t, "provisioning", out["step"])
	fnRef, ok := out["functionRef"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function-kcl", fnRef["name"])
	input, ok := out["input"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, input)
}

func TestStdlibRenderer_Render_BadYAML(t *testing.T) {
	files := fstest.MapFS{"notbad.kcl": &fstest.MapFile{Data: []byte("not: : valid")}}
	entry := StepEntry{
		Kind:      servicebundle.StepProvisioning,
		Function:  FunctionRef{Name: "function-kcl"},
		InputFile: "bad.kcl",
	}
	r := newRenderer(entry, files)
	_, err := r.Render(servicebundle.PipelineStep{Kind: servicebundle.StepProvisioning})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad.kcl")
}

func TestStdlibRenderer_Kind(t *testing.T) {
	entry := StepEntry{Kind: servicebundle.StepBackup}
	r := newRenderer(entry, nil)
	assert.Equal(t, servicebundle.StepBackup, r.Kind())
}
