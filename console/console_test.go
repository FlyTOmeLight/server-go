package console

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestConsoleProxyCapturesTargetAndAppCode(t *testing.T) {
	config := &conf.HTTPServerConfig{Listen: "first.example:8080"}
	proxy := NewConsoleProxy(config, 731)
	config.Listen = "second.example:8081"
	req := httptest.NewRequest(http.MethodPost, "http://frontend/example/api/v1/Create", strings.NewReader(`{}`))
	proxy.Director(req)
	if req.URL.Host != "first.example:8080" || req.URL.Path != "/example/internal/v1/Create" ||
		req.Method != http.MethodPost {
		t.Fatalf("incorrect proxy target: %s %s", req.Method, req.URL)
	}
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{},
		Request:    req,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}
	if err := proxy.ModifyResponse(resp); err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"statusCode":731500`) {
		t.Fatalf("missing explicit app code: status=%d body=%s", resp.StatusCode, body)
	}
}

func TestParseRequestRewritesAPISegment(t *testing.T) {
	for _, test := range []struct{ path, want string }{
		{path: "/example/api", want: "/example/internal"},
		{path: "/example/api/v1/Create", want: "/example/internal/v1/Create"},
		{path: "/example/apiv2/api/v1/Create", want: "/example/apiv2/internal/v1/Create"},
		{path: "/api/api/v1/Create", want: "/api/internal/v1/Create"},
		{path: "/example/apiv2/v1/Create", want: ""},
		{path: "/example/api-docs/v1/Create", want: ""},
		{path: "/api/v1/Create", want: ""},
	} {
		t.Run(test.path, func(t *testing.T) {
			path, err := parseRequest(httptest.NewRequest(http.MethodGet, test.path, nil))
			if path != test.want || (test.want == "" && err != serrors.ErrorInvalidURI) ||
				(test.want != "" && err != nil) {
				t.Fatalf("parseRequest = %q, %v", path, err)
			}
		})
	}
}

func TestConsoleProxyWrapsUpstreamResponses(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		gzip   bool
		code   string
	}{
		{name: "empty 404", status: http.StatusNotFound, code: "Not Found"},
		{name: "empty 500", status: http.StatusInternalServerError, code: "Internal Server Error"},
		{name: "JSON error", status: http.StatusConflict, body: `{"code":"Conflict"}`, code: "Conflict"},
		{name: "gzip JSON", status: http.StatusOK, body: `{"ok":true}`, gzip: true, code: "Success"},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept-Encoding") != "" || r.Header.Get("If-Modified-Since") != "" {
					t.Errorf("proxy forwarded compression or conditional request headers")
				}
				w.Header().Set(constants.HeaderRequestID, "upstream-request")
				w.Header().Set("Content-Type", "application/json")
				if test.gzip {
					w.Header().Set("Content-Encoding", "GZIP")
					w.Header().Set("Transfer-Encoding", "chunked")
				}
				w.WriteHeader(test.status)
				if test.gzip {
					compressed := gzip.NewWriter(w)
					_, err := io.WriteString(compressed, test.body)
					if err != nil {
						t.Error(err)
					}
					if err := compressed.Close(); err != nil {
						t.Error(err)
					}
				} else if _, err := io.WriteString(w, test.body); err != nil {
					t.Error(err)
				}
			}))
			defer backend.Close()
			proxy := NewConsoleProxy(
				&conf.HTTPServerConfig{Listen: strings.TrimPrefix(backend.URL, "http://")},
				731,
			)
			req := httptest.NewRequest(http.MethodGet, "/example/api/resource", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			req.Header.Set("If-Modified-Since", time.Now().UTC().Format(http.TimeFormat))
			recorder := httptest.NewRecorder()
			proxy.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusOK ||
				recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
				recorder.Header().Get("Content-Encoding") != "" ||
				recorder.Header().Get("Transfer-Encoding") != "" {
				t.Fatalf(
					"unexpected proxied response: %d %v %s",
					recorder.Code,
					recorder.Header(),
					recorder.Body,
				)
			}
			var response Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error == nil || response.Error.Code != test.code ||
				response.Error.RequestID != "upstream-request" {
				t.Fatalf("unexpected response envelope: %s", recorder.Body)
			}
			if test.status >= 400 && response.Error.StatusCode != 731000+test.status {
				t.Fatalf("unexpected application status code: %d", response.Error.StatusCode)
			}
			if test.body == "" && response.Error.Message != test.code {
				t.Fatalf("unexpected error message: %q", response.Error.Message)
			}
			if test.gzip {
				data, ok := response.Data.(map[string]any)
				if !ok || data["ok"] != true {
					t.Fatalf("unexpected gzip response data: %#v", response.Data)
				}
			}
		})
	}
}

func TestRegisterConsoleAPIForwardsMethods(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(r.Method + " " + r.URL.RequestURI()); err != nil {
			t.Error(err)
		}
	}))
	defer backend.Close()
	engine := gin.New()
	names := []string{"other", "spare capacity"}
	RegisterConsoleAPI(engine, &conf.Config{
		AppName: "example",
		HTTPServer: conf.HTTPServerConfig{
			Listen: strings.TrimPrefix(backend.URL, "http://"), ConsoleProxyNames: names[:1],
		},
	})
	if names[1] != "spare capacity" {
		t.Fatalf("registering console routes modified the caller's slice: %v", names)
	}
	for _, name := range []string{"example", "other"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(
				t.Context(),
				method,
				"/"+name+"/api/resource%20name?tag=a",
				nil,
			)
			engine.ServeHTTP(recorder, req)
			want := method + " /" + name + "/internal/resource%20name?tag=a"
			var response Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("%s: HTTP %d: %v", want, recorder.Code, err)
			}
			if recorder.Code != http.StatusOK || response.Data != want {
				t.Errorf("expected %q, got HTTP %d %s", want, recorder.Code, recorder.Body)
			}
		}
	}
}

func TestConsoleProxyStreamsSSEPastWriteDeadline(t *testing.T) {
	nextEvent := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-nextEvent:
			_, _ = io.WriteString(w, "data: second\n\n")
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
		}
	}))
	defer backend.Close()
	engine := gin.New()
	engine.GET("/example/api/*any", Reverse(&conf.HTTPServerConfig{
		Listen: strings.TrimPrefix(backend.URL, "http://"),
	}, 731, engine))
	frontend := httptest.NewUnstartedServer(engine)
	frontend.Config.WriteTimeout = 500 * time.Millisecond
	frontend.Start()
	defer frontend.Close()
	client := frontend.Client()
	client.Timeout = 5 * time.Second
	req, err := http.NewRequest(http.MethodGet, frontend.URL+"/example/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json, TEXT/EVENT-STREAM")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" {
		t.Fatalf("unexpected SSE response: %d %v", resp.StatusCode, resp.Header)
	}
	reader := bufio.NewReader(resp.Body)
	for _, expected := range []string{"data: first\n", "\n", "data: second\n", "\n"} {
		if expected == "data: second\n" {
			<-time.After(frontend.Config.WriteTimeout)
			close(nextEvent)
		}
		if got, err := reader.ReadString('\n'); err != nil || got != expected {
			t.Fatalf("stream line = %q, %v, want %q", got, err, expected)
		}
	}
}

func TestConsoleProxyReroutesWebSocket(t *testing.T) {
	engine := gin.New()
	engine.UseRawPath = true
	engine.GET("/example/internal/*any", func(c *core.Context) {
		if got := c.Request.URL; got.Path != "/example/internal/socket name/part" ||
			got.RawPath != "" || got.EscapedPath() != "/example/internal/socket%20name/part" {
			t.Errorf("unexpected rewritten URL: %#v", got)
		}
		c.Status(http.StatusNoContent)
	})
	engine.GET("/example/api/*any", Reverse(&conf.HTTPServerConfig{}, 731, engine), func(c *core.Context) {
		t.Error("continued running the original route after rerouting")
	})
	engine.GET("/missing/api/*any", Reverse(&conf.HTTPServerConfig{}, 731, nil))
	for path, status := range map[string]int{
		"/example/api/socket%20name%2Fpart": http.StatusNoContent,
		"/missing/api/socket":               http.StatusInternalServerError,
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "keep-alive, Upgrade")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		if recorder.Code != status {
			t.Errorf("%s: got HTTP %d, want %d", path, recorder.Code, status)
		}
	}
}

func TestConsoleProxyLogsExcludeCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy.log")
	log, err := logger.NewLogger(
		&logger.Config{Level: "debug", Outputs: []logger.Output{{Type: "file", Path: path}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	proxy := NewConsoleProxy(&conf.HTTPServerConfig{Listen: "example.test"}, 731)
	req := httptest.NewRequest(http.MethodGet, "/example/api/resource", nil)
	req = req.WithContext(logger.WithContext(req.Context(), log))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "key=secret-cookie")
	proxy.Director(req)
	proxy.ErrorHandler(httptest.NewRecorder(), req, errors.New("upstream failure"))
	log.Close()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"ProxyModifiedRequest", "ProxyError", "GET", "/example/internal/resource"} {
		if !strings.Contains(string(contents), expected) {
			t.Errorf("proxy log missing %q", expected)
		}
	}
	for _, secret := range []string{"Authorization", "secret-token", "Cookie", "secret-cookie"} {
		if strings.Contains(string(contents), secret) {
			t.Errorf("proxy log contains credential data: %q", secret)
		}
	}
}
