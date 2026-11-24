package middleware

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func Asset(assets []conf.HTTPServerAssetConfig, authEnable bool, engine *core.Engine) {
	protected := make(map[string]bool)
	for _, asset := range assets {
		if asset.AuthEnable && asset.FilePath != "" {
			protected[assetMount(asset.URLPath)] = true
		}
	}
	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		if route.Method == http.MethodGet || route.Method == http.MethodHead {
			registered[route.Path] = true
		}
	}
	for _, asset := range assets {
		mount := assetMount(asset.URLPath)
		pattern := path.Join(mount, "/*filepath")
		if asset.AuthEnable != authEnable || asset.FilePath == "" || registered[pattern] ||
			(!authEnable && protected[mount]) {
			continue
		}
		handler := func(c *core.Context) {
			c.Header("X-Content-Type-Options", "nosniff")
			c.Abort()
			requestPath := strings.TrimPrefix(c.Request.URL.Path, mount)
			for segment := range strings.SplitSeq(strings.ReplaceAll(requestPath, `\`, "/"), "/") {
				if strings.HasPrefix(segment, ".") {
					http.NotFound(c.Writer, c.Request)
					return
				}
			}
			root, err := os.OpenRoot(asset.FilePath)
			if err != nil {
				http.NotFound(c.Writer, c.Request)
				return
			}
			defer root.Close()
			filesystem := assetFiles{OnlyFilesFS: gin.OnlyFilesFS{FileSystem: http.FS(root.FS())}}
			fileServer := http.StripPrefix(mount, http.FileServer(filesystem))
			c.Writer.WriteHeader(http.StatusNotFound)
			fileServer.ServeHTTP(c.Writer, c.Request)
		}
		engine.GET(pattern, handler)
		engine.HEAD(pattern, handler)
		registered[pattern] = true
	}
}

type assetFiles struct{ gin.OnlyFilesFS }

func (files assetFiles) Open(name string) (http.File, error) {
	file, err := files.OnlyFilesFS.Open(name)
	if err != nil {
		return nil, fs.ErrNotExist
	}
	return file, nil
}

func assetMount(urlPath string) string {
	if urlPath == "" {
		return "/assets"
	}
	return path.Clean("/" + urlPath)
}
