package servicebundle

import (
	"encoding/json"
	"fmt"
)

// ServiceBundle is the maintainer-facing contract.
type ServiceBundle struct {
	// Meta contains meta information about this service bundle.
	// Information like the name, version, author and the stdlib repo.
	Meta Meta `json:"meta,omitempty"`

	// Claim declares the user-facing XRD / claim API: kind, shortNames and the
	// service-specific portion of the OpenAPI schema. Framework-wide fields
	// (size, monitoring toggles, …) are merged in by the stdlib.
	Claim *Claim `json:"claim,omitempty"`

	// Renderer declares how to produce the base set of resources (Helm chart
	// or a list of plain manifests). The pipeline's provisioning step calls
	// the renderer and applies the result.
	Renderer *Renderer `json:"renderer,omitempty"`

	// Pipeline is an ordered list of phases the framework executes per claim.
	// The maintainer chooses which phases to include and their order.
	Pipeline []PipelineStep `json:"pipeline,omitempty"`

	// Credentials declares the user-facing Secret in the claim's namespace.
	Credentials *Credentials `json:"credentials,omitempty"`

	// Plans are t-shirt-style sizing presets. The claim may reference a plan
	// by name; specific fields in spec.parameters.size override plan values.
	Plans map[string]map[string]string `json:"plans,omitempty"`

	// Lifecycle TBD
	// Lifecycle *Lifecycle `json:"lifecycle,omitempty"`
}

// Meta contains metadata about the servicebundle
// No omitempty here, these fields are all required.
type Meta struct {
	Name    string `json:"name"`
	Author  string `json:"author"`
	Version string `json:"version"`
	Stdlib  string `json:"stdlib"`
}

// Claim declares the user-facing API surface for this service.
type Claim struct {
	// Kind is the singular CamelCase kind of the claim resource (e.g.
	// VSHNPostgreSQL). The framework derives plural/lowercase forms from it.
	Kind string `json:"kind"`

	// ShortNames is the list of kubectl short names (e.g. ["vpg"]).
	ShortNames []string `json:"shortNames,omitempty"`

	// SimpleSchema is a kro SimpleSchema fragment. When set, it is
	// expanded into a full OpenAPI v3.
	SimpleSchema map[string]any `json:"simpleSchema,omitempty"`
}

type RendererType string

const (
	RendererTypeHelm           RendererType = "helm"
	RendererTypePlainManifests RendererType = "plain_manifests"
)

// Renderer is a tagged union: Type discriminates, Spec carries type-specific
// source fields. ValueMapping is type-independent and stays at renderer level.
type Renderer struct {
	Type         RendererType
	Spec         RendererSourceSpec
	ValueMapping []ValueMappingItem
}

// RendererSourceSpec is implemented by every concrete renderer source. New
// types: define a struct, implement RendererType(), register via
// RegisterRendererType.
type RendererSourceSpec interface {
	RendererType() RendererType
}

type HelmSource struct {
	Repository string `json:"repository"`
	Chart      string `json:"chart"`
	Version    string `json:"version"`
	// Values is a free-form set of static chart values applied before
	// valueMapping. Use it for static overrides (e.g. fullnameOverride,
	// global feature flags) that don't come from the claim.
	Values map[string]any `json:"values,omitempty"`
}

func (HelmSource) RendererType() RendererType { return RendererTypeHelm }

type PlainManifestsSource struct {
	// Templates is a map of inline manifests. Each entry is a single
	// Kubernetes resource. The framework applies them all at once.
	Templates map[string]any `json:"templates"`
}

func (PlainManifestsSource) RendererType() RendererType { return RendererTypePlainManifests }

var rendererRegistry = map[RendererType]func() RendererSourceSpec{
	RendererTypeHelm:           func() RendererSourceSpec { return &HelmSource{} },
	RendererTypePlainManifests: func() RendererSourceSpec { return &PlainManifestsSource{} },
}

// RegisterRendererType adds a new renderer type to the registry.
func RegisterRendererType(t RendererType, factory func() RendererSourceSpec) {
	rendererRegistry[t] = factory
}

func (r *Renderer) UnmarshalJSON(b []byte) error {
	t, spec, err := decodeTagged(b, "type", rendererRegistry, "renderer")
	if err != nil {
		return err
	}
	var extras struct {
		ValueMapping []ValueMappingItem `json:"valueMapping,omitempty"`
	}
	if err := json.Unmarshal(b, &extras); err != nil {
		return fmt.Errorf("renderer: %w", err)
	}
	r.Type, r.Spec, r.ValueMapping = t, spec, extras.ValueMapping
	return nil
}

func (r Renderer) MarshalJSON() ([]byte, error) {
	extra := map[string]json.RawMessage{}
	if len(r.ValueMapping) > 0 {
		vm, err := json.Marshal(r.ValueMapping)
		if err != nil {
			return nil, err
		}
		extra["valueMapping"] = vm
	}
	return encodeTagged(r.Spec, "type", r.Type, extra)
}

type ValueMappingItem struct {
	// ClaimPath is a JSONPath into the claim.
	ClaimPath string `json:"claimPath"`

	// Target is a JSONPath into the rendered manifest.
	Target string `json:"target"`

	// Manifest names the manifest to patch (must equal the rendered resource's
	// kind+name or an index, depending on renderer). Required when
	// plain_manifests has multiple resources.
	Manifest string `json:"manifest,omitempty"`
}

// ----- Pipeline -----

// PipelineStepKind names a phase. These can be extended later.
type PipelineStepKind string

const (
	StepProvisioning PipelineStepKind = "provisioning"
	StepNetworking   PipelineStepKind = "networking"
	StepBackup       PipelineStepKind = "backup"
	StepMonitoring   PipelineStepKind = "monitoring"
	StepMaintenance  PipelineStepKind = "maintenance"
	StepCustom       PipelineStepKind = "custom"
)

// PipelineStep kind defines the step and spec holds the specifics.
type PipelineStep struct {
	Kind PipelineStepKind
	Spec StepSpec
}

// StepSpec is the interface every concrete step kind implements. New kinds:
// define a struct, implement StepKind(), register via RegisterStepKind.
type StepSpec interface {
	StepKind() PipelineStepKind
}

// ProvisioningStep runs the renderer declared at ServiceBundle.Renderer.
type ProvisioningStep struct{}

func (ProvisioningStep) StepKind() PipelineStepKind { return StepProvisioning }

// NetworkingStep emits networking resources (cert-manager, etc.).
type NetworkingStep struct {
	// TLS, when present and Enabled, makes the framework emit a cert-manager
	// Issuer + Certificate in the instance namespace.
	TLS *TLSSpec `json:"tls,omitempty"`
}

func (NetworkingStep) StepKind() PipelineStepKind { return StepNetworking }

// BackupStep is a placeholder for backup-specific configuration.
type BackupStep struct{}

func (BackupStep) StepKind() PipelineStepKind { return StepBackup }

// MonitoringStep emits a PrometheusRule from the declared alerts.
type MonitoringStep struct {
	Alerts []Alert `json:"alerts,omitempty"`
}

func (MonitoringStep) StepKind() PipelineStepKind { return StepMonitoring }

// MaintenanceStep declares scheduled maintenance tasks.
type MaintenanceStep struct {
	DefaultSchedule string                      `json:"defaultSchedule,omitempty"`
	EnvFromClaim    map[string]ClaimPathBinding `json:"envFromClaim,omitempty"`
	Tasks           []MaintenanceTask           `json:"tasks,omitempty"`
}

func (MaintenanceStep) StepKind() PipelineStepKind { return StepMaintenance }

// CustomStep wraps an arbitrary Crossplane function. Input is passed verbatim.
type CustomStep struct {
	Function string         `json:"function"`
	Input    map[string]any `json:"input,omitempty"`
}

func (CustomStep) StepKind() PipelineStepKind { return StepCustom }

// stepRegistry maps kind → factory producing an empty Spec to decode into.
// Add new kinds with RegisterStepKind.
var stepRegistry = map[PipelineStepKind]func() StepSpec{
	StepProvisioning: func() StepSpec { return &ProvisioningStep{} },
	StepNetworking:   func() StepSpec { return &NetworkingStep{} },
	StepBackup:       func() StepSpec { return &BackupStep{} },
	StepMonitoring:   func() StepSpec { return &MonitoringStep{} },
	StepMaintenance:  func() StepSpec { return &MaintenanceStep{} },
	StepCustom:       func() StepSpec { return &CustomStep{} },
}

func (s *PipelineStep) UnmarshalJSON(b []byte) error {
	k, spec, err := decodeTagged(b, "kind", stepRegistry, "pipeline step")
	if err != nil {
		return err
	}
	s.Kind, s.Spec = k, spec
	return nil
}

func (s PipelineStep) MarshalJSON() ([]byte, error) {
	return encodeTagged(s.Spec, "kind", s.Kind, nil)
}

// Alert is one rule the framework emits inside a PrometheusRule. Templating:
// {{ $xr.metadata.namespace }} placeholders in Expr / Summary / Description /
// RunbookURL are rewritten to KCL ${oxr.metadata.namespace} at composition
// time so each alert is scoped to its instance namespace. Prometheus' own
// `{{ $labels.X }}` templating is passed through untouched.
type Alert struct {
	// Name is the alert identifier.
	Name string `json:"name"`

	// Expr is the PromQL expression. Required.
	Expr string `json:"expr"`

	// For is the optional duration the condition must hold before firing,
	// e.g. "5m" or "2h".
	For string `json:"for,omitempty"`

	// Severity is a shorthand that lands in labels.severity. Common values:
	// warning, critical, info.
	Severity string `json:"severity,omitempty"`

	// Labels merges with the (severity, syn) labels the framework always
	// emits.
	Labels map[string]string `json:"labels,omitempty"`

	// Summary is a short human-readable message; lands in annotations.summary.
	Summary string `json:"summary,omitempty"`

	// Description is the detailed explanation; lands in annotations.description.
	// Supports Prometheus' `{{ $labels.X }}` templating.
	Description string `json:"description,omitempty"`

	// RunbookURL lands in annotations.runbook_url.
	RunbookURL string `json:"runbookURL,omitempty"`

	// Annotations merges with summary/description/runbook_url.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TLSSpec configures the framework-emitted cert-manager Certificate. For PoC
// we only support self-signed issuers; later phases pick a cluster-installed
// Issuer based on capability detection.
type TLSSpec struct {
	// Enabled toggles the whole block. Default false.
	Enabled bool `json:"enabled,omitempty"`

	// SecretName is the name the cert-manager Certificate writes its
	// tls.crt / tls.key / ca.crt into. Required when Enabled.
	SecretName string `json:"secretName,omitempty"`

	// DNSNames listed on the Certificate. Each entry may use ${oxr.metadata.namespace}
	// style interpolation; the framework rewrites Go template `{{ $xr.X }}`
	// forms to KCL on emission.
	DNSNames []string `json:"dnsNames,omitempty"`

	// IssuerName overrides the framework's default ("appcat-self-signed").
	IssuerName string `json:"issuerName,omitempty"`
}

// ----- Maintenance tasks -----

// MaintenanceTaskType: default (framework-built-in) or custom (maintainer image).
type MaintenanceTaskType string

const (
	MaintenanceTaskDefault MaintenanceTaskType = "default"
	MaintenanceTaskCustom  MaintenanceTaskType = "custom"
)

// MaintenanceTask is one entry in pipeline.maintenance.tasks.
type MaintenanceTask struct {
	Type MaintenanceTaskType `json:"type"`

	// Name is the task name. For type=default, picks the built-in handler
	// (e.g. "patching-image"). For type=custom, used as the CronJob name suffix.
	Name string `json:"name"`

	// Schedule optionally overrides the pipeline's DefaultSchedule.
	Schedule string `json:"schedule,omitempty"`

	// Source is the container image for type=custom (e.g. "oci://ghcr.io/org/img:tag"
	// or a plain image reference). Ignored for type=default.
	Source string `json:"source,omitempty"`

	// Params is an opaque JSON object passed to the task handler. Built-in
	// handlers (type=default) document their expected fields.
	Params map[string]any `json:"params,omitempty"`
}

// ClaimPathBinding reads a string from the claim with optional default.
type ClaimPathBinding struct {
	ClaimPath string `json:"claimPath"`
	Default   string `json:"default,omitempty"`
}

// ----- Credentials -----

type Credentials struct {
	ValueMapping map[string]CredentialValue `json:"valueMapping"`
}

type CredentialSource string

const (
	CredSourceConst      CredentialSource = "const"
	CredSourceTemplate   CredentialSource = "template"
	CredSourceClaimParam CredentialSource = "claim_param"
	CredSourceSecretRef  CredentialSource = "secret_ref"
	CredSourceExpr       CredentialSource = "expr"
	CredSourceExec       CredentialSource = "exec"
)

// CredentialValue is a tagged union discriminated by Source.
type CredentialValue struct {
	Source CredentialSource
	Spec   CredentialSpec
}

// CredentialSpec is implemented by every concrete credential source. New
// sources: define a struct, implement CredentialSource(), register via
// RegisterCredentialSource.
type CredentialSpec interface {
	CredentialSource() CredentialSource
}

// CredConst is a literal value.
type CredConst struct {
	Value string `json:"value"`
}

func (CredConst) CredentialSource() CredentialSource { return CredSourceConst }

// CredTemplate is a Go-template string; may reference $xr (e.g.
// "{{ $xr.metadata.name }}-svc.{{ $xr.metadata.namespace }}.svc.cluster.local").
type CredTemplate struct {
	Value string `json:"value"`
}

func (CredTemplate) CredentialSource() CredentialSource { return CredSourceTemplate }

// CredClaimParam reads a path from the claim.
type CredClaimParam struct {
	Path string `json:"path"`
}

func (CredClaimParam) CredentialSource() CredentialSource { return CredSourceClaimParam }

// CredSecretRef references a key in an existing Secret in the instance namespace.
type CredSecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func (CredSecretRef) CredentialSource() CredentialSource { return CredSourceSecretRef }

// CredExpr is a CEL expression evaluated against the observed state.
type CredExpr struct {
	Expression string `json:"expression"`
}

func (CredExpr) CredentialSource() CredentialSource { return CredSourceExpr }

// CredExec runs a pod exec to obtain the secret value.
type CredExec struct {
	Pod       string            `json:"pod,omitempty"`
	Container string            `json:"container,omitempty"`
	Command   []string          `json:"command,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Parse     string            `json:"parse,omitempty"`
}

func (CredExec) CredentialSource() CredentialSource { return CredSourceExec }

var credentialRegistry = map[CredentialSource]func() CredentialSpec{
	CredSourceConst:      func() CredentialSpec { return &CredConst{} },
	CredSourceTemplate:   func() CredentialSpec { return &CredTemplate{} },
	CredSourceClaimParam: func() CredentialSpec { return &CredClaimParam{} },
	CredSourceSecretRef:  func() CredentialSpec { return &CredSecretRef{} },
	CredSourceExpr:       func() CredentialSpec { return &CredExpr{} },
	CredSourceExec:       func() CredentialSpec { return &CredExec{} },
}

// RegisterCredentialSource adds a new credential source to the registry.
func RegisterCredentialSource(s CredentialSource, factory func() CredentialSpec) {
	credentialRegistry[s] = factory
}

func (c *CredentialValue) UnmarshalJSON(b []byte) error {
	s, spec, err := decodeTagged(b, "source", credentialRegistry, "credential")
	if err != nil {
		return err
	}
	c.Source, c.Spec = s, spec
	return nil
}

func (c CredentialValue) MarshalJSON() ([]byte, error) {
	return encodeTagged(c.Spec, "source", c.Source, nil)
}
