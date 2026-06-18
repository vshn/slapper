// Package kcl emits Go values as KCL source-text literals.
package kcl

import (
	"fmt"
	"regexp"

	"kcl-lang.io/kcl-go/pkg/tools/gen"
)

// gen.Marshal emits empty containers as `{\n    \n}` / `[\n    \n]`. Rewrite to `{}` and `[]`.
var (
	emptyMap  = regexp.MustCompile(`\{\n\s*\}`)
	emptyList = regexp.MustCompile(`\[\n\s*\]`)
)

// Ref is a raw KCL expression.
type Ref struct {
	Expr string
}

// MarshalKcl is a custom KCL marshal hook.
// The marshaller would otherwise interpret
// the expression as an object, instead of a
// verbatim KCL expression.
func (r Ref) MarshalKcl() ([]byte, error) {
	return []byte(r.Expr), nil
}

func Emit(v any) (string, error) {
	rawKCL, err := gen.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshalling KCL: %w", err)
	}

	kcl := string(rawKCL)
	kcl = emptyList.ReplaceAllLiteralString(kcl, "[]")
	kcl = emptyMap.ReplaceAllLiteralString(kcl, "{}")

	return kcl, nil
}
