// Package template provides a structural placeholder resolver.
// We roll our own templating engine to avoid clashes with function
// inputs. Since they also use popular templating formats.
package template

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

const (
	slapperRef = "$slapperRef"
)

var slapperRefRegex = regexp.MustCompile(`\{\s*\` + slapperRef + `:\s*([^}\s]+)\s*}`)

// Resolver walks a parsed YAML tree and replaces every {$slapperRef: <path>}
// placeholder with the value returned by the matching Root.
type Resolver struct {
	roots map[string]Root
}

// New returns an empty Resolver. Register roots before calling Resolve.
func New() *Resolver {
	return &Resolver{roots: map[string]Root{}}
}

// Register adds a Root under the given name. The first dotted segment of a
// $slapperRef path selects the root; remaining segments are passed to
// Root.Resolve.
func (r *Resolver) Register(name string, root Root) {
	r.roots[name] = root
}

// Resolve walks tree in-place and returns it. Detection: a map with exactly
// one key "$slapperRef" whose value is a non-empty string is a placeholder.
// Resolved values are NOT re-walked.
//
// The top-level itself cannot be a placeholder: callers parse a YAML document
// whose root is always a mapping, so a top-level {$slapperRef: ...} would
// imply the entire document is replaced — out of scope.
func (r *Resolver) Resolve(tree map[string]any) (map[string]any, error) {
	out, err := r.walk(tree)
	if err != nil {
		return nil, err
	}
	return out.(map[string]any), nil
}

// walk handles every node type. Order matters: placeholder detection runs
// before generic map recursion so a single-key {$slapperRef: ...} map is
// not walked as a regular map first.
func (r *Resolver) walk(v any) (any, error) {
	switch n := v.(type) {
	case map[string]any:
		if len(n) == 1 {
			if raw, has := n[slapperRef]; has {
				path, ok := raw.(string)
				if !ok {
					return nil, fmt.Errorf("$slapperRef value must be string, got %T", raw)
				}
				if path == "" {
					return nil, fmt.Errorf("$slapperRef path must not be empty")
				}
				return r.resolveRef(path)
			}
		}
		for k, child := range n {
			replaced, err := r.walk(child)
			if err != nil {
				return nil, err
			}
			n[k] = replaced
		}
		return n, nil

	case []any:
		for i, child := range n {
			replaced, err := r.walk(child)
			if err != nil {
				return nil, err
			}
			n[i] = replaced
		}
		return n, nil

	case string:
		var resolveErr error
		out := slapperRefRegex.ReplaceAllStringFunc(n, func(match string) string {
			if resolveErr != nil {
				return match
			}
			path := slapperRefRegex.FindStringSubmatch(match)[1]
			resolved, err := r.resolveRef(path)
			if err != nil {
				resolveErr = err
				return match
			}
			return fmt.Sprint(resolved)
		})
		return out, resolveErr

	default:
		return v, nil
	}
}

// resolveRef splits the dotted path, dispatches to the matching root, and
// returns the resolved value verbatim. Unknown root → hard error.
func (r *Resolver) resolveRef(path string) (any, error) {
	parts := strings.Split(path, ".")
	rootName := parts[0]
	root, ok := r.roots[rootName]
	if !ok {
		slog.Debug("unknown $slapperRef root", "root", rootName, "path", path)
		return nil, fmt.Errorf("unknown $slapperRef root %q in %q", rootName, path)
	}
	return root.Resolve(parts[1:])
}
