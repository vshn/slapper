package stdlib

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/opencontainers/go-digest"
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
	ref, err := name.ParseReference(src.Ref)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid stdlib reference: %w", err)
	}

	repoCacheRoot := filepath.Join(
		src.CacheDir,
		ref.Context().RegistryStr(),
		ref.Context().RepositoryStr(),
	)

	opts := []crane.Option{
		crane.WithContext(ctx),
		crane.WithAuthFromKeychain(authn.DefaultKeychain),
	}
	if src.PlainHTTP {
		opts = append(opts, crane.Insecure)
	}

	// If the ref carries a digest and the digest-keyed cache is populated,
	// short-circuit the network. Tag-only refs always Head.
	if d, ok := ref.(name.Digest); ok {
		if parsed, err := digest.Parse(d.DigestStr()); err == nil {
			cached := filepath.Join(repoCacheRoot, parsed.String())
			if isPopulated(cached) {
				slog.Info(
					"stdlib resolved",
					"source", "oci",
					"ref", src.Ref,
					"digest", parsed.String(),
					"cachedHit", true,
				)
				return loadLocal(cached)
			}
		}
	}

	desc, err := crane.Head(src.Ref, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("image descriptor: %w", err)
	}

	dgst := desc.Digest.String()
	cacheDir := filepath.Join(repoCacheRoot, dgst)

	// skip pulling again if the digest is already present in the cache
	if isPopulated(cacheDir) {
		slog.Info(
			"stdlib resolved",
			"source", "oci",
			"ref", src.Ref,
			"digest", dgst,
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
		if err := os.RemoveAll(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Error("removing tmp dir", "error", err)
		}
	}()

	img, err := crane.Pull(src.Ref, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("oci pull: %w", err)
	}

	rc := mutate.Extract(img)
	defer func() {
		err := rc.Close()
		if err != nil {
			slog.Error("closing OCI image reader", "error", err)
		}
	}()

	if err := untar(rc, tmp); err != nil {
		return nil, nil, fmt.Errorf("extract image: %w", err)
	}

	if err := os.Rename(tmp, cacheDir); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, nil, fmt.Errorf("rename image cache %s: %w", cacheDir, err)
	}

	slog.Info(
		"stdlib resolved",
		"source", "oci",
		"ref", src.Ref,
		"digest", dgst,
		"cachedHit", false,
	)
	return loadLocal(cacheDir)
}

func isPopulated(cacheDir string) bool {
	_, err := os.Stat(filepath.Join(cacheDir, manifestFile))
	return !errors.Is(err, os.ErrNotExist)
}

// untar extracts a tar stream into dst. Rejects path traversal and absolute
// paths; skips symlinks, hardlinks, devices, and other non-regular entries.
func untar(r io.Reader, dst string) error {
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("resolve dst: %w", err)
	}
	dstPrefix := dstAbs + string(os.PathSeparator)

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar entry: %w", err)
		}

		clean := filepath.Clean(hdr.Name)
		if clean == "." || clean == "" {
			continue
		}
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("tar entry escapes dst: %q", hdr.Name)
		}

		target := filepath.Join(dstAbs, clean)
		if target != dstAbs && !strings.HasPrefix(target, dstPrefix) {
			return fmt.Errorf("tar entry escapes dst: %q", hdr.Name)
		}

		mode := os.FileMode(hdr.Mode) & 0o777

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, mode|0o700); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent of %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o600)
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				err := f.Close()
				if err != nil {
					slog.Error("closing file", "file", f.Name(), "error", err)
				}
				return fmt.Errorf("write %s: %w", target, err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("close %s: %w", target, err)
			}
		default:
			// skip symlinks, hardlinks, char/block devices, fifos, etc.
			continue
		}
	}
}

// DefaultCacheDir returns the default cache root for OCI-pulled stdlib
// artifacts: <user-cache>/slapper/stdlib. Wired in as the default for the
// CLI's --stdlib-cache-dir flag.
func DefaultCacheDir() string {
	h, _ := os.UserCacheDir()
	return filepath.Join(h, "slapper", "stdlib")
}
