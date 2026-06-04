package servicebundle

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// roundTrip decodes raw YAML into a fresh T, re-marshals it, decodes that into
// another T, and asserts both decoded values are equal. Catches both Unmarshal
// and Marshal regressions in one shot.
func roundTrip[T any](t *testing.T, raw string) T {
	t.Helper()
	var first T
	if err := yaml.Unmarshal([]byte(raw), &first); err != nil {
		t.Fatalf("first unmarshal: %v", err)
	}
	out, err := yaml.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var second T
	if err := yaml.Unmarshal(out, &second); err != nil {
		t.Fatalf("second unmarshal of %s: %v", out, err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("round-trip mismatch:\n first  = %#v\n second = %#v\n second YAML = %s", first, second, out)
	}
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
				if _, ok := spec.(*ProvisioningStep); !ok {
					t.Fatalf("got %T", spec)
				}
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
				if n.TLS == nil || !n.TLS.Enabled || n.TLS.SecretName != "my-tls" {
					t.Fatalf("bad TLS: %+v", n.TLS)
				}
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
				if len(m.Alerts) != 1 || m.Alerts[0].Name != "HighCPU" {
					t.Fatalf("bad alerts: %+v", m.Alerts)
				}
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
				if m.DefaultSchedule != "0 3 * * *" || len(m.Tasks) != 1 {
					t.Fatalf("bad maintenance: %+v", m)
				}
			},
		},
		{
			name: "custom",
			yaml: `
kind: custom
function: ghcr.io/foo/bar:v1
input:
  apiVersion: x/v1
  kind: Script
  script: |
    print(1)
`,
			wantKind: StepCustom,
			check: func(t *testing.T, spec StepSpec) {
				c := spec.(*CustomStep)
				if c.Function != "ghcr.io/foo/bar:v1" {
					t.Fatalf("bad function: %q", c.Function)
				}
				if c.Input["kind"] != "Script" {
					t.Fatalf("input not preserved: %+v", c.Input)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step := roundTrip[PipelineStep](t, tc.yaml)
			if step.Kind != tc.wantKind {
				t.Fatalf("kind: got %q want %q", step.Kind, tc.wantKind)
			}
			tc.check(t, step.Spec)
		})
	}
}

func TestPipelineStep_UnknownKind(t *testing.T) {
	var s PipelineStep
	err := yaml.Unmarshal([]byte(`kind: made-up`), &s)
	if err == nil || !strings.Contains(err.Error(), `unknown kind made-up`) {
		t.Fatalf("want unknown-kind error, got %v", err)
	}
}

func TestPipelineStep_MissingKind(t *testing.T) {
	var s PipelineStep
	err := yaml.Unmarshal([]byte(`function: foo`), &s)
	if err == nil || !strings.Contains(err.Error(), `missing "kind"`) {
		t.Fatalf("want missing-kind error, got %v", err)
	}
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
	if r.Type != RendererTypeHelm {
		t.Fatalf("type: %q", r.Type)
	}
	h := r.Spec.(*HelmSource)
	if h.Repository != "https://charts.example/" || h.Chart != "my-chart" || h.Version != "1.2.3" {
		t.Fatalf("bad helm: %+v", h)
	}
	if got, ok := h.Values["replicaCount"]; !ok || got != float64(3) {
		t.Fatalf("values not preserved: %+v", h.Values)
	}
	if len(r.ValueMapping) != 1 || r.ValueMapping[0].ClaimPath != ".spec.size" {
		t.Fatalf("bad valueMapping: %+v", r.ValueMapping)
	}
}

func TestRenderer_PlainManifests(t *testing.T) {
	in := `
type: plain_manifests
templates:
  a:
    kind: ConfigMap
`
	r := roundTrip[Renderer](t, in)
	if r.Type != RendererTypePlainManifests {
		t.Fatalf("type: %q", r.Type)
	}
	p := r.Spec.(*PlainManifestsSource)
	if _, ok := p.Templates["a"]; !ok {
		t.Fatalf("templates not preserved: %+v", p.Templates)
	}
}

func TestRenderer_OmitsEmptyValueMapping(t *testing.T) {
	r := Renderer{
		Type: RendererTypeHelm,
		Spec: &HelmSource{Repository: "r", Chart: "c", Version: "v"},
	}
	out, err := yaml.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "valueMapping") {
		t.Fatalf("expected no valueMapping key, got %s", out)
	}
}

func TestRenderer_UnknownType(t *testing.T) {
	var r Renderer
	err := yaml.Unmarshal([]byte(`type: "???"`), &r)
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("want unknown-type error, got %v", err)
	}
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
				if s.(*CredConst).Value != "literal" {
					t.Fatal("bad const")
				}
			},
		},
		{
			"template",
			`{source: template, value: "{{ $xr.metadata.name }}"}`,
			CredSourceTemplate,
			func(t *testing.T, s CredentialSpec) {
				if s.(*CredTemplate).Value == "" {
					t.Fatal("bad template")
				}
			},
		},
		{
			"claim_param",
			`{source: claim_param, path: .spec.foo}`,
			CredSourceClaimParam,
			func(t *testing.T, s CredentialSpec) {
				if s.(*CredClaimParam).Path != ".spec.foo" {
					t.Fatal("bad claim_param")
				}
			},
		},
		{
			"secret_ref",
			`{source: secret_ref, name: my-secret, key: password}`,
			CredSourceSecretRef,
			func(t *testing.T, s CredentialSpec) {
				c := s.(*CredSecretRef)
				if c.Name != "my-secret" || c.Key != "password" {
					t.Fatalf("bad secret_ref: %+v", c)
				}
			},
		},
		{
			"expr",
			`{source: expr, expression: observed.cluster.status.host}`,
			CredSourceExpr,
			func(t *testing.T, s CredentialSpec) {
				if s.(*CredExpr).Expression == "" {
					t.Fatal("bad expr")
				}
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
				if e.Pod != "p" || e.Container != "c" || len(e.Command) != 3 || e.Env["K"] != "V" || e.Parse != "json" {
					t.Fatalf("bad exec: %+v", e)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cv := roundTrip[CredentialValue](t, tc.yaml)
			if cv.Source != tc.wantSource {
				t.Fatalf("source: got %q want %q", cv.Source, tc.wantSource)
			}
			tc.check(t, cv.Spec)
		})
	}
}

func TestCredentialValue_UnknownSource(t *testing.T) {
	var cv CredentialValue
	err := yaml.Unmarshal([]byte(`source: nope`), &cv)
	if err == nil || !strings.Contains(err.Error(), "unknown source") {
		t.Fatalf("want unknown-source error, got %v", err)
	}
}

// TestExampleBundle parses examples/servicebundle.yaml. Guards against drift
// between the example and the type definitions.
func TestExampleBundle(t *testing.T) {
	raw, err := os.ReadFile("../../examples/servicebundle.yaml")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	var sb ServiceBundle
	if err := yaml.Unmarshal(raw, &sb); err != nil {
		t.Fatalf("decode example: %v", err)
	}
	if sb.Meta.Name == "" {
		t.Fatal("example missing meta.name")
	}
	if sb.Claim == nil || sb.Claim.Kind == "" {
		t.Fatal("example missing claim.kind")
	}
	if sb.Renderer == nil {
		t.Fatal("example missing renderer")
	}
	if len(sb.Pipeline) == 0 {
		t.Fatal("example missing pipeline steps")
	}
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
    function: ghcr.io/foo/fn:v1
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
	if err := yaml.Unmarshal([]byte(raw), &sb); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sb.Meta.Name != "pg" {
		t.Fatalf("meta: %+v", sb.Meta)
	}
	if sb.Renderer == nil || sb.Renderer.Type != RendererTypeHelm {
		t.Fatalf("renderer: %+v", sb.Renderer)
	}
	if len(sb.Pipeline) != 3 {
		t.Fatalf("pipeline len: %d", len(sb.Pipeline))
	}
	if sb.Pipeline[1].Kind != StepCustom {
		t.Fatalf("pipeline[1] kind: %q", sb.Pipeline[1].Kind)
	}
	if _, ok := sb.Pipeline[2].Spec.(*MaintenanceStep); !ok {
		t.Fatalf("pipeline[2] spec: %T", sb.Pipeline[2].Spec)
	}
	host := sb.Credentials.ValueMapping["host"]
	if host.Source != CredSourceExpr || host.Spec.(*CredExpr).Expression == "" {
		t.Fatalf("host cred: %+v", host)
	}
}
