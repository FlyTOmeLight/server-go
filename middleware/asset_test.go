package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/FlyTOmeLight/server-go/conf"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestAssetsConfinePathsAndHideDotfiles(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "public")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o700))
	for filename, content := range map[string]string{
		"public/file.txt": "public", "public/.hidden": "hidden", "public/.git/info": "hidden",
		"private.txt": "private",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o600))
	}
	require.NoError(t, os.Symlink("file.txt", filepath.Join(root, "inside.txt")))
	require.NoError(t, os.Symlink("../private.txt", filepath.Join(root, "outside.txt")))
	engine := gin.New()
	Asset([]conf.HTTPServerAssetConfig{
		{FilePath: root}, {FilePath: dir, URLPath: "/assets/"},
	}, false, engine)
	require.Len(t, engine.Routes(), 2)
	for _, test := range []struct {
		path, method, body string
		status             int
	}{
		{path: "/assets/file.txt", method: http.MethodGet, body: "public", status: http.StatusOK},
		{path: "/assets/inside.txt", method: http.MethodGet, body: "public", status: http.StatusOK},
		{path: "/assets/file.txt", method: http.MethodHead, status: http.StatusOK},
		{path: "/assets/.hidden", method: http.MethodGet, status: http.StatusNotFound},
		{path: "/assets/.git/info", method: http.MethodGet, status: http.StatusNotFound},
		{path: "/assets/outside.txt", method: http.MethodGet, status: http.StatusNotFound},
	} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"), test.path)
		require.Equal(t, test.status, response.Code, test.path)
		if test.status == http.StatusOK {
			require.Equal(t, test.body, response.Body.String(), test.path)
		}
	}
}

func TestDuplicateAssetMountUsesProtectedRoute(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("file"), 0o600))
	assets := []conf.HTTPServerAssetConfig{
		{FilePath: root, URLPath: "/shared"},
		{FilePath: root, URLPath: "/shared", AuthEnable: true},
	}
	engine := gin.New()
	Asset(assets, false, engine)
	engine.Use(func(c *core.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	Asset(assets, true, engine)
	Asset(assets, true, engine)
	require.Len(t, engine.Routes(), 2)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/shared/file.txt", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}
