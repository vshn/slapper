package stdlib

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	fakeRepoName      = "test/stdlib"
	fakeTag           = "v0"
	manifestMediaType = "application/vnd.oci.image.manifest.v1+json"
	configMediaType   = "application/vnd.oci.image.config.v1+json"
	layerMediaType    = "application/vnd.oci.image.layer.v1.tar"
)

type ociDescriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type fakeArtifact struct {
	manifestJSON   []byte
	manifestDigest string
	blobs          map[string][]byte
}

// buildArtifact walks fixtureDir, packs all files into a single tar layer,
// builds an OCI image manifest, and returns the manifest bytes + a
// digest→bytes blob map.
func buildArtifact(t *testing.T, fixtureDir string) fakeArtifact {
	t.Helper()
	blobs := map[string][]byte{}

	add := func(content []byte) (digest string, size int64) {
		sum := sha256.Sum256(content)
		d := "sha256:" + hex.EncodeToString(sum[:])
		blobs[d] = content
		return d, int64(len(content))
	}

	// ggcr's image validation requires rootfs.diff_ids to match layer
	// uncompressed digests. Build the tar first so we can plug the digest
	// into the config.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	err := filepath.WalkDir(fixtureDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(fixtureDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		hdr := &tar.Header{
			Name:     rel,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if writeErr := tw.WriteHeader(hdr); writeErr != nil {
			return writeErr
		}
		_, writeErr := tw.Write(content)
		return writeErr
	})
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NotZero(t, tarBuf.Len(), "fixture %q produced empty tar", fixtureDir)

	layerDigest, layerSize := add(tarBuf.Bytes())

	cfg := map[string]any{
		"architecture": "amd64",
		"os":           "linux",
		"rootfs": map[string]any{
			"type":     "layers",
			"diff_ids": []string{layerDigest},
		},
	}
	configBytes, err := json.Marshal(cfg)
	require.NoError(t, err)
	configDigest, configSize := add(configBytes)

	manifest := map[string]any{
		"schemaVersion": 2,
		"mediaType":     manifestMediaType,
		"config": ociDescriptor{
			MediaType: configMediaType,
			Digest:    configDigest,
			Size:      configSize,
		},
		"layers": []ociDescriptor{{
			MediaType: layerMediaType,
			Digest:    layerDigest,
			Size:      layerSize,
		}},
	}
	mJSON, err := json.Marshal(manifest)
	require.NoError(t, err)
	mDigest, _ := add(mJSON)

	return fakeArtifact{
		manifestJSON:   mJSON,
		manifestDigest: mDigest,
		blobs:          blobs,
	}
}

// fakeRegistry stands up an httptest server serving a minimal subset of the
// OCI distribution-spec endpoints needed to satisfy `oras.Copy`:
//
//	GET  /v2/
//	HEAD /v2/<name>/manifests/<tag|digest>
//	GET  /v2/<name>/manifests/<tag|digest>
//	HEAD /v2/<name>/blobs/<digest>
//	GET  /v2/<name>/blobs/<digest>
//
// The artifact is published under repository "test/stdlib" at tag "v0".
// Returns the server (caller may Close early for the cache-hit test) and the
// manifest digest so tests can build digest-pinned refs.
func fakeRegistry(t *testing.T, fixtureDir string) (*httptest.Server, string) {
	t.Helper()
	art := buildArtifact(t, fixtureDir)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

		path := strings.TrimPrefix(r.URL.Path, "/v2/")
		if path == "" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if rest, ok := strings.CutPrefix(path, fakeRepoName+"/manifests/"); ok {
			if rest != fakeTag && rest != art.manifestDigest {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", manifestMediaType)
			w.Header().Set("Docker-Content-Digest", art.manifestDigest)
			w.Header().Set("Content-Length", strconv.Itoa(len(art.manifestJSON)))
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = w.Write(art.manifestJSON)
			return
		}

		if rest, ok := strings.CutPrefix(path, fakeRepoName+"/blobs/"); ok {
			blob, found := art.blobs[rest]
			if !found {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Docker-Content-Digest", rest)
			w.Header().Set("Content-Length", strconv.Itoa(len(blob)))
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = w.Write(blob)
			return
		}

		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, art.manifestDigest
}

// httpStub404 returns a handler that answers the /v2/ ping but 404s every
// manifest/blob request — used by TestOCISource_Load_PullError to exercise
// the failure path of the OCI loader.
func httpStub404() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	})
}
