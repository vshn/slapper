package servicebundle

import (
	"encoding/json"
	"fmt"
	"maps"
)

// decodeTagged decodes a JSON object that carries a discriminator field. The
// discriminator's value selects a factory from registry; the whole body is then
// re-decoded into the produced spec.
func decodeTagged[K comparable, S any](
	b []byte,
	discriminator string,
	registry map[K]func() S,
	label string,
) (K, S, error) {
	var zeroK K
	var zeroS S
	head := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &head); err != nil {
		return zeroK, zeroS, fmt.Errorf("%s: %w", label, err)
	}
	rawKind, ok := head[discriminator]
	if !ok {
		return zeroK, zeroS, fmt.Errorf("%s: missing %q", label, discriminator)
	}
	var k K
	if err := json.Unmarshal(rawKind, &k); err != nil {
		return zeroK, zeroS, fmt.Errorf("%s: %w", label, err)
	}
	factory, ok := registry[k]
	if !ok {
		return zeroK, zeroS, fmt.Errorf("%s: unknown %s %v", label, discriminator, k)
	}
	spec := factory()
	if err := json.Unmarshal(b, spec); err != nil {
		return zeroK, zeroS, fmt.Errorf("%s %v: %w", label, k, err)
	}
	return k, spec, nil
}

// encodeTagged emits spec as a JSON object and merges the discriminator field
// plus any extra renderer-level fields supplied by the caller.
func encodeTagged[K any](
	spec any,
	discriminator string,
	kind K,
	extra map[string]json.RawMessage,
) ([]byte, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	m := map[string]json.RawMessage{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
	}
	kRaw, err := json.Marshal(kind)
	if err != nil {
		return nil, err
	}
	m[discriminator] = kRaw
	maps.Copy(m, extra)
	return json.Marshal(m)
}
