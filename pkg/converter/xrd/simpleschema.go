package xrd

import (
	"encoding/json"
	"fmt"

	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
	"github.com/vshn/slapper/pkg/servicebundle"
)

// expandSimpleSchema converts a kro SimpleSchema fragment into an OpenAPI v3
// schema represented as a generic map suitable for embedding in an
// unstructured XRD object.
func expandSimpleSchema(simple map[string]any) (map[string]any, error) {
	props, err := simpleschema.ToOpenAPISpec(simple, nil)
	if err != nil {
		return nil, fmt.Errorf("simpleschema expansion failed: %w", err)
	}

	raw, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal expanded schema: %w", err)
	}

	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal expanded schema: %w", err)
	}
	return out, nil
}

// frameworkSchema returns the default schema for any xrd.
// The service maintainer can only change the service stanze, according to the
// definition in the serviceBundle.
// TODO: maybe should live with the stdlib?
func frameworkSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "User-controlled parameters. Framework owns plan/instances/size/maintenance; service maintainer owns the service block.",
		"properties": map[string]any{
			// TODO: fill from the plans
			"plan": map[string]any{
				"type":        "string",
				"description": "Sizing plan name (small, medium, large). Resolves to default cpu/memory/disk values; explicit fields in 'size' override.",
				"default":     "small",
				"enum":        []any{"small", "medium", "large"},
			},
			// TODO: expose this
			"instances": map[string]any{
				"type":        "integer",
				"description": "Number of workload instances/replicas. Service maintainer decides what 'instance' means (e.g. PostgreSQL nodes, Redis replicas).",
				"default":     int64(1),
				"minimum":     int64(1),
			},
			// TODO: implement maintenance
			"maintenance": map[string]any{
				"type":        "object",
				"description": "Controls when framework-managed maintenance tasks run.",
				"properties": map[string]any{
					"schedule": map[string]any{
						"type":        "string",
						"description": "Cron expression for the maintenance window (e.g. '0 3 * * 0' = Sundays at 03:00 UTC).",
						"default":     "0 3 * * 0",
					},
				},
			},
			// "service" filled in dynamically from sb.spec.claim.serviceSchemaSimple.
		},
	}
}

func buildParameterSchema(sb *servicebundle.ServiceBundle) (map[string]any, error) {
	full := frameworkSchema()
	props, ok := full["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("framework schema missing properties map")
	}

	serviceSchema := map[string]any{
		// Default if the maintainer doesn't ship a service block.
		"type":        "object",
		"description": "Service-specific parameters defined by the service maintainer.",
	}
	if sb.Claim != nil && sb.Claim.SimpleSchema != nil {
		expanded, err := expandSimpleSchema(sb.Claim.SimpleSchema)
		if err != nil {
			return nil, err
		}
		serviceSchema = expanded

		// Preserve maintainer description if set, otherwise inject framework default.
		if _, ok := serviceSchema["description"]; !ok {
			serviceSchema["description"] = "Service-specific parameters defined by the service maintainer."
		}
	}
	props["service"] = serviceSchema
	return full, nil
}
