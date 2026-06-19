package stdlib

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ociHost(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "http://")
}

func TestOCISource_Load_FromRegistry(t *testing.T) {
	srv, digest := fakeRegistry(t, "testdata")
	_ = digest

	cacheDir := t.TempDir()
	src := OCISource{
		Ref:       ociHost(srv) + "/test/stdlib:v0",
		CacheDir:  cacheDir,
		PlainHTTP: true,
	}

	m, files, err := Load(context.Background(), src)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "test-stdlib", m.Metadata.Name)

	f, err := files.Open("templates/provisioning-helm.yaml")
	require.NoError(t, err)
	defer f.Close()
}

func TestOCISource_Load_CacheHitSkipsNetwork(t *testing.T) {
	srv, digest := fakeRegistry(t, "testdata")
	cacheDir := t.TempDir()
	src := OCISource{
		Ref:       ociHost(srv) + "/test/stdlib@" + digest,
		CacheDir:  cacheDir,
		PlainHTTP: true,
	}

	_, _, err := Load(context.Background(), src)
	require.NoError(t, err)

	srv.Close()

	_, _, err = Load(context.Background(), src)
	require.NoError(t, err, "second load should hit cache, not network")
}

func TestOCISource_Load_PullError(t *testing.T) {
	srv := httptest.NewServer(httpStub404())
	defer srv.Close()
	src := OCISource{
		Ref:       strings.TrimPrefix(srv.URL, "http://") + "/missing:v0",
		CacheDir:  t.TempDir(),
		PlainHTTP: true,
	}
	_, _, err := Load(context.Background(), src)
	require.Error(t, err)
}
