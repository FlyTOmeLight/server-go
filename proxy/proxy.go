package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func Reverse(rawURL string) core.HandlerFunc {
	return func(c *core.Context) {
		targetURL, _ := url.Parse(rawURL)

		director := func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetURL.Path
		}

		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// ReverseEndpoint
// For example:
// rawURL is localhost:80/api/v1
// sourcePrefix is /demo
// the url 'localhost:3000/demo/path1/path2' will be replaced to 'localhost:80/api/v1/path1/path2'
func ReverseEndpoint(rawURL string, sourcePrefix ...string) core.HandlerFunc {
	return func(c *core.Context) {
		targetURL, _ := url.Parse(rawURL)

		var prefix string
		if len(sourcePrefix) > 0 {
			prefix = sourcePrefix[0]
		}

		director := func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host

			path := req.URL.Path
			if prefix != "" && strings.HasPrefix(path, prefix) {
				path = strings.TrimPrefix(path, prefix)
			}
			req.URL.Path = targetURL.Path + path
		}

		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func ReverseWithDirector(director func(r *http.Request)) core.HandlerFunc {
	return ReverseWithModifyHandler(director, nil)
}

func ReverseWithModifyHandler(
	director func(r *http.Request),
	modifyResponse func(*http.Response) error,
) core.HandlerFunc {
	proxy := &httputil.ReverseProxy{
		Director:       director,
		ModifyResponse: modifyResponse,
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			logger.FromContext(request.Context()).
				Errorf(request.Context(), "ProxyError req: %#v, err: %v", request, err)
			writer.WriteHeader(http.StatusBadGateway)
		},
	}
	return ReverseWithProxy(proxy)
}

func ReverseWithProxy(proxy *httputil.ReverseProxy) core.HandlerFunc {
	return func(c *core.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
