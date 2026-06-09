package stdlib

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

	cacheDir := filepath.Join(src.CacheDir, ref.Registry, ref.Repository)

	// if there's a digest and we have it locally,
	// we can skip the registry
	d, err := digest.Parse(ref.Reference)
	if err == nil {
		cacheDir := filepath.Join(cacheDir, d.String())
		if isPopulated(cacheDir) {
			return loadLocal(cacheDir)
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

	cacheDir = filepath.Join(cacheDir, dgst.Digest.String())

	tmp, err := os.MkdirTemp("", "slapper-stdlib-*")
	if err != nil {
		return nil, nil, fmt.Errorf("cache tmp dir: %w", err)
	}

	defer os.RemoveAll(tmp)

	store, err := file.New(tmp)
	if err != nil {
		return nil, nil, fmt.Errorf("cache tmp store: %w", err)
	}
	defer store.Close()

	_, err = oras.Copy(ctx, repo, ref.Reference, store, ref.ReferenceOrDefault(), oras.DefaultCopyOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("oras image copy: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create image cache %s; %w", tmp, err)
	}

	if err := os.Rename(tmp, cacheDir); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, nil, fmt.Errorf("rename image cache %s: %w", cacheDir, err)
	}

	return loadLocal(cacheDir)
}

func isPopulated(cacheDir string) bool {
	_, err := os.Stat(cacheDir + "/" + manifestFile)
	return !errors.Is(err, os.ErrNotExist)
}

// DefaultCacheDir gets the defaul cache dir for
// the current user/os
// TODO: wire in
func DefaultCacheDir() string {
	h, _ := os.UserCacheDir()
	return filepath.Join(h, "slapper", "stdlib")
}
