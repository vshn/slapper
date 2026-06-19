package servicebundle

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// roundTrip decodes raw YAML into a fresh T, re-marshals it, decodes that into
// another T, and asserts both decoded values are equal. Catches both Unmarshal
// and Marshal regressions in one shot.
func roundTrip[T any](t *testing.T, raw string) T {
	t.Helper()
	var first T
	require.NoError(t, yaml.Unmarshal([]byte(raw), &first), "first unmarshal")

	out, err := yaml.Marshal(first)
	require.NoError(t, err, "marshal")

	var second T
	require.NoError(t, yaml.Unmarshal(out, &second), "second unmarshal of %s", out)
	require.Equal(t, first, second, "round-trip mismatch; second YAML = %s", out)
	return first
}

func TestPipelineStep_RoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		yaml     string
		wantKind PipelineStepKind
		check    func(t *testing.T, spec StepSpec)
	}{
		{
			name:     "provisioning",
			yaml:     `kind: provisioning`,
			wantKind: StepProvisioning,
			check: func(t *testing.T, spec StepSpec) {
				_, ok := spec.(*ProvisioningStep)
				assert.True(t, ok, "got %T", spec)
			},
		},
		{
			name: "networking with TLS",
			yaml: `
kind: networking
tls:
  enabled: true
  secretName: my-tls
  dnsNames:
    - a.example
`,
			wantKind: StepNetworking,
			check: func(t *testing.T, spec StepSpec) {
				n := spec.(*NetworkingStep)
				require.NotNil(t, n.TLS, "TLS nil")
				assert.True(t, n.TLS.Enabled)
				assert.Equal(t, "my-tls", n.TLS.SecretName)
			},
		},
		{
			name: "monitoring",
			yaml: `
kind: monitoring
alerts:
  - name: HighCPU
    expr: rate(cpu[5m])>0.9
`,
			wantKind: StepMonitoring,
			check: func(t *testing.T, spec StepSpec) {
				m := spec.(*MonitoringStep)
				require.Len(t, m.Alerts, 1)
				assert.Equal(t, "HighCPU", m.Alerts[0].Name)
			},
		},
		{
			name: "maintenance",
			yaml: `
kind: maintenance
defaultSchedule: "0 3 * * *"
tasks:
  - type: default
    name: patching-image
`,
			wantKind: StepMaintenance,
			check: func(t *testing.T, spec StepSpec) {
				m := spec.(*MaintenanceStep)
				assert.Equal(t, "0 3 * * *", m.DefaultSchedule)
				assert.Len(t, m.Tasks, 1)
			},
		},
		{
			name: "custom",
			yaml: `
kind: custom
function:
  name: ghcr.io/foo/bar
  versionConstraint: v1
input:
  apiVersion: x/v1
  kind: Script
  script: |
    print(1)
`,
			wantKind: StepCustom,
			check: func(t *testing.T, spec StepSpec) {
				c := spec.(*CustomStep)
				assert.Equal(t, "ghcr.io/foo/bar", c.Function.Name)
				assert.Equal(t, "v1", c.Function.VersionConstraint)
				assert.Equal(t, "Script", c.Input["kind"], "input not preserved")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step := roundTrip[PipelineStep](t, tc.yaml)
			require.Equal(t, tc.wantKind, step.Kind)
			tc.check(t, step.Spec)
		})
	}
}

func TestPipelineStep_UnknownKind(t *testing.T) {
	var s PipelineStep
	err := yaml.Unmarshal([]byte(`kind: made-up`), &s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown kind made-up")
}

func TestPipelineStep_MissingKind(t *testing.T) {
	var s PipelineStep
	err := yaml.Unmarshal([]byte(`function: foo`), &s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `missing "kind"`)
}

func TestRenderer_Helm(t *testing.T) {
	in := `
type: helm
repository: https://charts.example/
chart: my-chart
version: 1.2.3
values:
  replicaCount: 3
valueMapping:
  - claimPath: .spec.size
    target: .spec.replicas
`
	r := roundTrip[Renderer](t, in)
	require.Equal(t, RendererTypeHelm, r.Type)

	h := r.Spec.(*HelmSource)
	assert.Equal(t, "https://charts.example/", h.Repository)
	assert.Equal(t, "my-chart", h.Chart)
	assert.Equal(t, "1.2.3", h.Version)
	assert.Equal(t, float64(3), h.Values["replicaCount"], "values not preserved")

	require.Len(t, r.ValueMapping, 1)
	assert.Equal(t, ".spec.size", r.ValueMapping[0].ClaimPath)
}

func TestRenderer_PlainManifests(t *testing.T) {
	in := `
type: plain_manifests
templates:
  a:
    kind: ConfigMap
`
	r := roundTrip[Renderer](t, in)
	require.Equal(t, RendererTypePlainManifests, r.Type)

	p := r.Spec.(*PlainManifestsSource)
	_, ok := p.Templates["a"]
	assert.True(t, ok, "templates not preserved: %+v", p.Templates)
}

func TestRenderer_OmitsEmptyValueMapping(t *testing.T) {
	r := Renderer{
		Type: RendererTypeHelm,
		Spec: &HelmSource{Repository: "r", Chart: "c", Version: "v"},
	}
	out, err := yaml.Marshal(r)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "valueMapping", "expected no valueMapping key")
}

func TestRenderer_UnknownType(t *testing.T) {
	var r Renderer
	err := yaml.Unmarshal([]byte(`type: "???"`), &r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type")
}

func TestCredentialValue_AllSources(t *testing.T) {
	cases := []struct {
		name       string
		yaml       string
		wantSource CredentialSource
		check      func(t *testing.T, spec CredentialSpec)
	}{
		{
			"const",
			`{source: const, value: literal}`,
			CredSourceConst,
			func(t *testing.T, s CredentialSpec) {
				assert.Equal(t, "literal", s.(*CredConst).Value)
			},
		},
		{
			"template",
			`{source: template, value: "{{ $xr.metadata.name }}"}`,
			CredSourceTemplate,
			func(t *testing.T, s CredentialSpec) {
				assert.NotEmpty(t, s.(*CredTemplate).Value)
			},
		},
		{
			"claim_param",
			`{source: claim_param, path: .spec.foo}`,
			CredSourceClaimParam,
			func(t *testing.T, s CredentialSpec) {
				assert.Equal(t, ".spec.foo", s.(*CredClaimParam).Path)
			},
		},
		{
			"secret_ref",
			`{source: secret_ref, name: my-secret, key: password}`,
			CredSourceSecretRef,
			func(t *testing.T, s CredentialSpec) {
				c := s.(*CredSecretRef)
				assert.Equal(t, "my-secret", c.Name)
				assert.Equal(t, "password", c.Key)
			},
		},
		{
			"expr",
			`{source: expr, expression: observed.cluster.status.host}`,
			CredSourceExpr,
			func(t *testing.T, s CredentialSpec) {
				assert.NotEmpty(t, s.(*CredExpr).Expression)
			},
		},
		{
			"exec",
			`
source: exec
pod: p
container: c
command: [sh, -c, "echo hi"]
env:
  K: V
parse: json
`,
			CredSourceExec,
			func(t *testing.T, s CredentialSpec) {
				e := s.(*CredExec)
				assert.Equal(t, "p", e.Pod)
				assert.Equal(t, "c", e.Container)
				assert.Len(t, e.Command, 3)
				assert.Equal(t, "V", e.Env["K"])
				assert.Equal(t, "json", e.Parse)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cv := roundTrip[CredentialValue](t, tc.yaml)
			require.Equal(t, tc.wantSource, cv.Source)
			tc.check(t, cv.Spec)
		})
	}
}

func TestCredentialValue_UnknownSource(t *testing.T) {
	var cv CredentialValue
	err := yaml.Unmarshal([]byte(`source: nope`), &cv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source")
}

// TestExampleBundle parses examples/servicebundle.yaml. Guards against drift
// between the example and the type definitions.
func TestExampleBundle(t *testing.T) {
	raw, err := os.ReadFile("../../examples/servicebundle.yaml")
	require.NoError(t, err, "read example")

	var sb ServiceBundle
	require.NoError(t, yaml.Unmarshal(raw, &sb), "decode example")

	assert.NotEmpty(t, sb.Meta.Name, "example missing meta.name")
	require.NotNil(t, sb.Claim, "example missing claim")
	assert.NotEmpty(t, sb.Claim.Kind, "example missing claim.kind")
	assert.NotNil(t, sb.Renderer, "example missing renderer")
	assert.NotEmpty(t, sb.Pipeline, "example missing pipeline steps")
}

func TestServiceBundle_Decode(t *testing.T) {
	raw := `
meta:
  name: pg
  author: vshn
  version: 0.1.0
  stdlib: ghcr.io/vshn/stdlib
renderer:
  type: helm
  repository: https://charts.example/
  chart: pg
  version: 1.0.0
pipeline:
  - kind: provisioning
  - kind: custom
    function:
      name: ghcr.io/foo/fn
      versionConstraint: v1
    input:
      x: 1
  - kind: maintenance
    schedule: ""
credentials:
  valueMapping:
    host:
      source: expr
      expression: observed.cluster.status.host
    pass:
      source: secret_ref
      name: creds
      key: password
`

	var sb ServiceBundle
	require.NoError(t, yaml.Unmarshal([]byte(raw), &sb), "decode")

	assert.Equal(t, "pg", sb.Meta.Name)

	require.NotNil(t, sb.Renderer)
	assert.Equal(t, RendererTypeHelm, sb.Renderer.Type)

	require.Len(t, sb.Pipeline, 3)
	assert.Equal(t, StepCustom, sb.Pipeline[1].Kind)

	_, ok := sb.Pipeline[2].Spec.(*MaintenanceStep)
	assert.True(t, ok, "pipeline[2] spec: %T", sb.Pipeline[2].Spec)

	host := sb.Credentials.ValueMapping["host"]
	assert.Equal(t, CredSourceExpr, host.Source)
	assert.NotEmpty(t, host.Spec.(*CredExpr).Expression)
}

// ----- Renderer / HelmSource Validate -----

func TestHelmSource_Validate_OK(t *testing.T) {
	h := HelmSource{Repository: "r", Chart: "c", Version: "v"}
	assert.NoError(t, h.Validate())
}

func TestHelmSource_Validate_MissingRepository(t *testing.T) {
	h := HelmSource{Chart: "c", Version: "v"}
	err := h.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}

func TestHelmSource_Validate_MissingChart(t *testing.T) {
	h := HelmSource{Repository: "r", Version: "v"}
	err := h.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chart")
}

func TestHelmSource_Validate_MissingVersion(t *testing.T) {
	h := HelmSource{Repository: "r", Chart: "c"}
	err := h.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestHelmSource_Validate_AllMissingReportsAll(t *testing.T) {
	h := HelmSource{}
	err := h.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
	assert.Contains(t, err.Error(), "chart")
	assert.Contains(t, err.Error(), "version")
}

func TestRenderer_Validate_Helm_OK(t *testing.T) {
	r := &Renderer{
		Type: RendererTypeHelm,
		Spec: &HelmSource{Repository: "r", Chart: "c", Version: "v"},
	}
	assert.NoError(t, r.Validate())
}

func TestRenderer_Validate_Helm_DelegatesToSpec(t *testing.T) {
	r := &Renderer{
		Type: RendererTypeHelm,
		Spec: &HelmSource{},
	}
	err := r.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}

func TestRenderer_Validate_UnknownTypeErrors(t *testing.T) {
	r := &Renderer{Type: RendererType("nope"), Spec: nil}
	err := r.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func TestRenderer_Validate_SpecTypeMismatch(t *testing.T) {
	// Type says helm but Spec is plain_manifests.
	r := &Renderer{Type: RendererTypeHelm, Spec: &PlainManifestsSource{}}
	err := r.Validate()
	require.Error(t, err)
}
