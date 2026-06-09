package stdlib

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

// OCISource pulls from an OCI registry, caching by digest.
type OCISource struct {
	Ref       string
	CacheDir  string
	PlainHTTP bool
}

func (o OCISource) source() string {
	return "oci:" + o.Ref
}

func loadOCI(ctx context.Context, src OCISource) (*Manifest, fs.FS, error) {
	ref, err := registry.ParseReference(src.Ref)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid stdlib reference: %w", err)
	}

	repoCacheRoot := filepath.Join(src.CacheDir, ref.Registry, ref.Repository)

	// If the ref carries a digest and the digest-keyed cache is populated,
	// short-circuit the network. Tag-only refs always Resolve.
	if d, err := digest.Parse(ref.Reference); err == nil {
		cached := filepath.Join(repoCacheRoot, d.String())
		if isPopulated(cached) {
			slog.Info(
				"stdlib resolved",
				"source", "oci",
				"ref", src.Ref,
				"digest", d.String(),
				"cachedHit", true,
			)
			return loadLocal(cached)
		}
	}

	repo, err := remote.NewRepository(ref.String())
	if err != nil {
		return nil, nil, fmt.Errorf("can't open remote registry: %w", err)
	}

	repo.PlainHTTP = src.PlainHTTP

	dgst, err := repo.Resolve(ctx, ref.Reference)
	if err != nil {
		return nil, nil, fmt.Errorf("image descriptor: %w", err)
	}

	cacheDir := filepath.Join(repoCacheRoot, dgst.Digest.String())

	// skip pulling again, if the digest is already present
	// in the cache
	if isPopulated(cacheDir) {
		slog.Info(
			"stdlib resolved",
			"source", "oci",
			"ref", src.Ref,
			"digest", dgst.Digest.String(),
			"cachedHit", true,
		)
		return loadLocal(cacheDir)
	}

	if err := os.MkdirAll(repoCacheRoot, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create cache root: %w", err)
	}
	tmp, err := os.MkdirTemp(repoCacheRoot, ".slapper-stdlib-*")
	if err != nil {
		return nil, nil, fmt.Errorf("cache tmp dir: %w", err)
	}

	defer func() {
		err := os.RemoveAll(tmp)
		if err != nil {
			slog.Error("removing tmp dir", "error", err)
		}
	}()

	// file.new has a 32Mb limit as of oras 2.6.1
	// since stdlibs are mostly some yamls, this should be
	// plenty.
	store, err := file.New(tmp)
	if err != nil {
		return nil, nil, fmt.Errorf("cache tmp store: %w", err)
	}

	defer func() {
		err := store.Close()
		if err != nil {
			slog.Error("closing filestore", "error", err)
		}
	}()

	_, err = oras.Copy(ctx, repo, ref.Reference, store, ref.ReferenceOrDefault(), oras.DefaultCopyOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("oras image copy: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create image cache %s: %w", cacheDir, err)
	}

	if err := os.Rename(tmp, cacheDir); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, nil, fmt.Errorf("rename image cache %s: %w", cacheDir, err)
	}

	slog.Info(
		"stdlib resolved",
		"source", "oci",
		"ref", src.Ref,
		"digest", dgst.Digest.String(),
		"cachedHit", false,
	)
	return loadLocal(cacheDir)
}

func isPopulated(cacheDir string) bool {
	_, err := os.Stat(filepath.Join(cacheDir, manifestFile))
	return !errors.Is(err, os.ErrNotExist)
}

// DefaultCacheDir returns the default cache root for OCI-pulled stdlib
// artifacts: <user-cache>/slapper/stdlib. Wired in as the default for the
// CLI's --stdlib-cache-dir flag.
func DefaultCacheDir() string {
	h, _ := os.UserCacheDir()
	return filepath.Join(h, "slapper", "stdlib")
}
