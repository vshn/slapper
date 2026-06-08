package stdlib

import (
	"context"
	"fmt"
	"io/fs"
)

const (
	constLocalSource = "LocalSource"
	constOCISource   = "OCISource"
	manifestFile     = "stdlib.yaml"
)

// Source identifies where to load the stdlib from.
type Source interface {
	source() string
}

// LocalSource loads from a directory on disk
type LocalSource string

func (l LocalSource) source() string {
	return "local:" + string(l)
}

// OCISource pulls from an OCI registry, caching by digest.
type OCISource struct {
	Ref       string
	CacheDir  string
	PlainHTTP bool
}

func (o OCISource) source() string {
	return "oci:" + o.Ref
}

// Load loads the stdlib according to the source
func Load(ctx context.Context, src Source) (*Manifest, fs.FS, error) {
	switch s := src.(type) {
	case LocalSource:
		return loadLocal(string(s))
	case OCISource:
		// return loadOCI(ctx, s)
	case nil:
		return nil, nil, fmt.Errorf("nil stdlib source")
	default:
		return nil, nil, fmt.Errorf("unknown stdlib source type %T", src)
	}
	return nil, nil, fmt.Errorf("invald stdlib type: %T", src)
}
