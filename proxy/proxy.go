package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"

	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

var sharedTransport = &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	ExpectContinueTimeout: time.Second,
}

type Option func(*httputil.ReverseProxy)

// WithTransport supplies a reusable transport for outbound requests.
func WithTransport(transport http.RoundTripper) Option {
	return func(proxy *httputil.ReverseProxy) {
		if transport != nil {
			proxy.Transport = transport
		}
	}
}

// NewReverseProxy appends the request path to the target's base path.
func NewReverseProxy(rawURL string, opts ...Option) (core.HandlerFunc, error) {
	target, err := parseTarget(rawURL)
	if err != nil {
		return nil, err
	}
	return newProxy(target, "", false, false, opts...), nil
}

func Reverse(rawURL string) core.HandlerFunc {
	target, err := parseTarget(rawURL)
	if err != nil {
		return invalidTarget(http.StatusInternalServerError)
	}
	return newProxy(target, "", true, true)
}

// ReverseEndpoint removes sourcePrefix from the request path before appending
// it to the target's base path. The prefix must match whole path segments.
func ReverseEndpoint(rawURL string, sourcePrefix ...string) core.HandlerFunc {
	target, err := parseTarget(rawURL)
	if err != nil {
		return invalidTarget(http.StatusBadRequest)
	}
	var prefix string
	if len(sourcePrefix) > 0 {
		prefix = strings.TrimRight(sourcePrefix[0], "/")
	}
	return newProxy(target, prefix, false, true)
}

func parseTarget(rawURL string) (*url.URL, error) {
	target, err := url.Parse(rawURL)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") ||
		target.Hostname() == "" || target.User != nil || target.Fragment != "" {
		return nil, errors.New(
			"proxy target must be an absolute HTTP(S) URL without user information or a fragment",
		)
	}
	return target, nil
}

func invalidTarget(status int) core.HandlerFunc {
	return func(c *core.Context) {
		serrors.AbortWithError(c, serrors.NewError(status, "InvalidProxyTarget", "invalid proxy target URL"))
	}
}

func newProxy(target *url.URL, sourcePrefix string, fixedPath, legacy bool, opts ...Option) core.HandlerFunc {
	if legacy {
		target.RawQuery, target.ForceQuery = "", false
	}
	proxy := &httputil.ReverseProxy{
		Transport:    sharedTransport,
		ErrorHandler: proxyError,
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(target)
			request.SetXForwarded()
			request.Out.Host = target.Host
			if legacy {
				request.Out.Host = request.In.Host
			}
			incoming := ""
			if !fixedPath {
				incoming = request.In.URL.Path
				if sourcePrefix != "" &&
					(incoming == sourcePrefix || strings.HasPrefix(incoming, sourcePrefix+"/")) {
					incoming = strings.TrimPrefix(incoming, sourcePrefix)
				}
			}
			request.Out.URL.Path = joinPath(target.Path, incoming)
			request.Out.URL.RawPath = ""
		},
	}
	for _, option := range opts {
		option(proxy)
	}
	return ReverseWithProxy(proxy)
}

func joinPath(base, incoming string) string {
	base = strings.ReplaceAll(base, `\`, "/")
	incoming = strings.ReplaceAll(incoming, `\`, "/")
	joined := path.Join(path.Clean("/"+base), path.Clean("/"+incoming))
	if joined != "/" && (strings.HasSuffix(incoming, "/") || (incoming == "" && strings.HasSuffix(base, "/"))) {
		joined += "/"
	}
	return joined
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
		ErrorHandler:   proxyError,
	}
	return ReverseWithProxy(proxy)
}

func proxyError(writer http.ResponseWriter, request *http.Request, err error) {
	var urlError *url.Error
	if errors.As(err, &urlError) {
		err = urlError.Err
	}
	logger.FromContext(request.Context()).
		Errorf(request.Context(), "proxy error: method=%s path=%q error=%v", request.Method, request.URL.Path, err)
	writer.WriteHeader(http.StatusBadGateway)
}

func ReverseWithProxy(proxy *httputil.ReverseProxy) core.HandlerFunc {
	return func(c *core.Context) {
		c.Abort()
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
