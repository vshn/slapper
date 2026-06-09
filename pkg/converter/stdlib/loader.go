package stdlib

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
)

const manifestFile = "stdlib.yaml"

// Source identifies where to load the stdlib from.
type Source interface {
	source() string
}

// LocalSource loads from a directory on disk
type LocalSource string

func (l LocalSource) source() string {
	return "local:" + string(l)
}

// Load loads the stdlib according to the source. On success it logs
// "stdlib resolved" with source/ref/digest/cachedHit attributes; the OCI
// loader emits its own log so this wrapper only logs for LocalSource.
func Load(ctx context.Context, src Source) (*Manifest, fs.FS, error) {
	switch s := src.(type) {
	case LocalSource:
		m, files, err := loadLocal(string(s))
		if err == nil {
			slog.Info("stdlib resolved",
				"source", "local",
				"ref", string(s),
				"digest", "",
				"cachedHit", false,
			)
		}
		return m, files, err
	case OCISource:
		return loadOCI(ctx, s)
	case nil:
		return nil, nil, fmt.Errorf("nil stdlib source")
	default:
		return nil, nil, fmt.Errorf("unknown stdlib source type %T", src)
	}
}
