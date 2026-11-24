package version

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetVersionByURL(t *testing.T) {
	want := VersionInfo{
		AppVersion:    "1.2.3",
		GitBranch:     "main",
		GitCommit:     "abc123",
		BuildDate:     "2026-11-24",
		BuildPipeline: "ci",
		BuildNumber:   "42",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/version" {
			http.NotFound(w, request)
			return
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()

	for _, test := range []struct {
		name     string
		endpoint string
	}{
		{name: "HTTP 200", endpoint: server.URL},
		{name: "trailing slash and whitespace", endpoint: " " + server.URL + "///\t"},
		{name: "bare host and port", endpoint: strings.TrimPrefix(server.URL, "http://")},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := GetVersionByURL(test.endpoint)
			require.NoError(t, err)
			require.Equal(t, &want, got)
		})
	}
}

func TestGetVersionByURLContextErrors(t *testing.T) {
	t.Run("empty endpoint", func(t *testing.T) {
		got, err := GetVersionByURLContext(t.Context(), " \t")
		require.ErrorContains(t, err, "version endpoint is empty")
		require.Nil(t, got)
	})

	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "non-200 status", status: http.StatusServiceUnavailable, body: "{}"},
		{name: "malformed JSON", status: http.StatusOK, body: "not JSON"},
		{name: "response exceeds decoding limit", status: http.StatusOK, body: strings.Repeat(" ", 1<<20) + "{}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			got, err := GetVersionByURLContext(t.Context(), server.URL)
			require.Error(t, err)
			require.Nil(t, got)
			if test.status != http.StatusOK {
				require.ErrorContains(t, err, "503")
			}
		})
	}

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		got, err := GetVersionByURLContext(ctx, "127.0.0.1:1")
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, got)
	})
}

func TestVersionHandler(t *testing.T) {
	router := gin.New()
	router.GET("/version", Version())
	writer := httptest.NewRecorder()
	router.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/version", nil))

	require.Equal(t, http.StatusOK, writer.Code)
	var got VersionInfo
	require.NoError(t, json.Unmarshal(writer.Body.Bytes(), &got))
	require.Equal(t, GetVersion(), &got)
}
