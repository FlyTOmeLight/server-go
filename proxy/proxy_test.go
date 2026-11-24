package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (transport transportFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestReverseProxyRewritesAndAborts(t *testing.T) {
	handler, err := NewReverseProxy("https://upstream.example/base?fixed=yes", WithTransport(transportFunc(
		func(request *http.Request) (*http.Response, error) {
			require.Equal(t, "/base/nested/item/", request.URL.Path)
			require.Equal(t, "fixed=yes&client=ok", request.URL.RawQuery)
			require.Equal(t, "upstream.example", request.Host)
			require.Equal(t, "client.example", request.Header.Get("X-Forwarded-Host"))
			require.Equal(t, "192.0.2.1", request.Header.Get("X-Forwarded-For"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		},
	)))
	require.NoError(t, err)
	engine := gin.New()
	engine.GET("/*rest", handler, func(*core.Context) { t.Fatal("continued after proxy response") })
	request := httptest.NewRequest(http.MethodGet, "https://client.example/nested/item/?client=ok", nil)
	request = request.WithContext(t.Context())
	request.Header.Set("X-Forwarded-Host", "untrusted.example")
	request.Header.Set("X-Forwarded-For", "198.51.100.1")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "ok", response.Body.String())
}

func TestJoinPathKeepsBase(t *testing.T) {
	require.Equal(t, "/base/items", joinPath("/base", "/draft/../items"))
	require.Equal(t, "/base/items", joinPath("/base", "/../items"))
	require.Equal(t, "/base/items", joinPath("/base", `\..\items`))
}

func TestProxyRejectsInvalidTargets(t *testing.T) {
	for _, target := range []string{"%zz", "/relative", "ftp://host", "http://", "http://name:password@host"} {
		_, err := NewReverseProxy(target)
		require.Error(t, err)
	}
	for _, test := range []struct {
		handler core.HandlerFunc
		status  int
	}{
		{handler: Reverse("%zz"), status: http.StatusInternalServerError},
		{handler: ReverseEndpoint("%zz"), status: http.StatusBadRequest},
	} {
		engine := gin.New()
		engine.GET("/", test.handler, func(*core.Context) { t.Fatal("continued after invalid target") })
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, test.status, response.Code)
	}
}

func TestProxyErrorLogsMethodAndPathWithoutCredentials(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "proxy.log")
	log, err := logger.NewLogger(&logger.Config{Outputs: []logger.Output{{Type: "File", Path: logPath}}})
	require.NoError(t, err)
	defer log.Close()
	handler, err := NewReverseProxy("https://upstream.example/base", WithTransport(transportFunc(
		func(request *http.Request) (*http.Response, error) {
			return nil, &url.Error{
				Op:  "GET",
				URL: request.URL.String(),
				Err: errors.New("upstream unavailable"),
			}
		},
	)))
	require.NoError(t, err)
	engine := gin.New()
	engine.GET("/data", handler)
	request := httptest.NewRequest(http.MethodGet, "/data?token=query-secret", nil)
	request = request.WithContext(logger.WithContext(t.Context(), log))
	request.Header.Set("Authorization", "Bearer header-secret")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadGateway, response.Code)
	logged, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Contains(t, string(logged), "method=GET")
	require.Contains(t, string(logged), "/base/data")
	require.Contains(t, string(logged), "upstream unavailable")
	require.NotContains(t, string(logged), "secret")
}

func TestLegacyProxyPaths(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, request.URL.RequestURI())
	}))
	defer upstream.Close()
	for _, test := range []struct {
		handler core.HandlerFunc
		in, out string
	}{
		{handler: Reverse(upstream.URL + "/fixed"), in: "/demo/file?keep=1", out: "/fixed?keep=1"},
		{handler: ReverseEndpoint(upstream.URL+"/base/", "/demo"), in: "/demo/file/", out: "/base/file/"},
		{handler: ReverseEndpoint(upstream.URL+"/base", "/demo"), in: "/demo-extra/file", out: "/base/demo-extra/file"},
	} {
		engine := gin.New()
		engine.GET("/*rest", test.handler, func(*core.Context) { t.Fatal("continued after proxy response") })
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, test.in, nil).WithContext(t.Context())
		engine.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, test.out, response.Body.String())
	}
}
