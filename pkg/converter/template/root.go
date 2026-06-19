package template

// Root resolves a dotted path (already split, root segment removed) into a
// concrete Go value. Roots own their own optionality semantics: missing
// optional fields return (zeroValue, nil); missing required fields return
// an error.
type Root interface {
	Resolve(path []string) (any, error)
}
