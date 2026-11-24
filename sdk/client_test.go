package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"resty.dev/v3"

	"github.com/FlyTOmeLight/server-go/constants"
)

func TestClonePreservesSettingsAndResourceOwnership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("X-Default") != "default" ||
			r.Header.Get("X-Signature") != "signed" ||
			r.URL.Query().Get("key") != "value" {
			t.Errorf("clone lost request settings: headers=%v query=%v", r.Header, r.URL.Query())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	config := &Config{
		Endpoint:    server.URL,
		Timeout:     3,
		Retry:       2,
		Credentials: &Credentials{AuthType: AuthTypeAPIKey, APIKey: "token"},
	}
	client, err := NewClientWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	client.SetHeader("X-Default", "default").SetQueryParam("key", "value")
	client.AddRequestMiddleware(func(_ *resty.Client, request *resty.Request) error {
		request.SetHeader("X-Signature", "signed")
		return nil
	})
	clone := client.Clone()
	if clone.Timeout() != 3*time.Second || clone.RetryCount() != 2 || clone.Transport() != client.Transport() {
		t.Fatal("clone lost timeout, retry or transport")
	}
	config.Credentials.APIKey = "changed"
	clone.Config.Credentials.APIKey = "clone"
	if client.Config.Credentials.APIKey != "token" {
		t.Fatal("client config aliases caller or clone credentials")
	}
	if err := clone.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := clone.R().SetContext(context.Background()).Get("/"); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		unsupported bool
	}{
		{name: "nil"},
		{name: "empty endpoint", config: &Config{}},
		{name: "relative endpoint", config: &Config{Endpoint: "localhost"}},
		{name: "negative retry", config: &Config{Endpoint: "http://localhost", Retry: -1}},
		{name: "relative proxy", config: &Config{Endpoint: "http://localhost", ProxyURL: "localhost:8080"}},
		{
			name:   "missing proxy scheme",
			config: &Config{Endpoint: "http://localhost", ProxyURL: "//localhost:8080"},
		},
		{name: "missing proxy host", config: &Config{Endpoint: "http://localhost", ProxyURL: "http://:8080"}},
		{name: "malformed proxy", config: &Config{Endpoint: "http://localhost", ProxyURL: "http://%"}},
		{
			name:   "unsupported proxy scheme",
			config: &Config{Endpoint: "http://localhost", ProxyURL: "ftp://localhost"},
		},
		{
			name: "empty API key",
			config: &Config{
				Endpoint:    "http://localhost",
				Credentials: &Credentials{AuthType: AuthTypeAPIKey},
			},
		},
		{
			name: "AKSK",
			config: &Config{
				Endpoint: "http://localhost",
				Credentials: &Credentials{
					AuthType:        AuthTypeAKSK,
					AccessKeyId:     "ak",
					SecretAccessKey: "sk",
				},
			},
			unsupported: true,
		},
		{
			name: "unknown auth",
			config: &Config{
				Endpoint:    "http://localhost",
				Credentials: &Credentials{AuthType: "unknown"},
			},
			unsupported: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClientWithConfig(test.config)
			if err == nil || client != nil {
				t.Fatalf("got client=%v err=%v", client, err)
			}
			if test.unsupported && !errors.Is(err, ErrUnsupportedAuth) {
				t.Fatalf("expected unsupported authentication error, got %v", err)
			}
		})
	}
}

func TestNewClientAcceptsSupportedProxySchemes(t *testing.T) {
	for _, proxy := range []string{"http://localhost:8080", "https://localhost:443", "socks5://localhost:1080"} {
		t.Run(proxy, func(t *testing.T) {
			client, err := NewClientWithConfig(&Config{Endpoint: "http://localhost", ProxyURL: proxy})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if got := client.ProxyURL(); got == nil || got.String() != proxy {
				t.Fatalf("proxy = %v, want %s", got, proxy)
			}
		})
	}
}

func TestCloseNilClient(t *testing.T) {
	var client *Client
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientPropagatesRequestID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header()[constants.HeaderRequestID] = r.Header.Values(constants.HeaderRequestID)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := NewClientWithConfig(&Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, test := range []struct {
		name      string
		contextID any
		explicit  bool
		verbatim  bool
		want      string
	}{
		{name: "context ID", contextID: "inbound", want: "inbound"},
		{name: "explicit header", contextID: "inbound", explicit: true, want: "override"},
		{name: "explicit empty header", contextID: "inbound", explicit: true},
		{name: "verbatim header", contextID: "inbound", explicit: true, verbatim: true, want: "override"},
		{name: "empty context ID", contextID: ""},
		{name: "wrong context value type", contextID: 123},
		{name: "no context ID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := client.R()
			if test.contextID != nil {
				request.SetContext(
					context.WithValue(context.Background(), constants.KeyRequestID, test.contextID),
				)
			}
			if test.verbatim {
				request.SetHeaderVerbatim("request-id", test.want)
			} else if test.explicit {
				request.SetHeader("request-id", test.want)
			}
			response, err := request.Get("/")
			if err != nil {
				t.Fatal(err)
			}
			got := response.Header().Values(constants.HeaderRequestID)
			wantCount := 0
			if test.want != "" || test.explicit {
				wantCount = 1
			}
			if len(got) != wantCount || response.Header().Get(constants.HeaderRequestID) != test.want {
				t.Fatalf(
					"downstream request ID = %q, want %q (explicit = %v)",
					got,
					test.want,
					test.explicit,
				)
			}
		})
	}
}

func TestClientRetriesOnlyIdempotentRequests(t *testing.T) {
	for _, test := range []struct {
		name, method, path, key, token string
		attempts                       int32
	}{
		{name: "GET", method: http.MethodGet, attempts: 2},
		{name: "HEAD", method: http.MethodHead, attempts: 2},
		{name: "OPTIONS", method: http.MethodOptions, attempts: 2},
		{name: "PUT", method: http.MethodPut, attempts: 2},
		{name: "DELETE", method: http.MethodDelete, attempts: 2},
		{name: "TRACE", method: http.MethodTrace, attempts: 1},
		{name: "plain POST", method: http.MethodPost, attempts: 1},
		{name: "plain PATCH", method: http.MethodPatch, attempts: 1},
		{name: "POST with query parameter", method: http.MethodPost, token: "token", attempts: 2},
		{name: "PATCH with URL token", method: http.MethodPatch, path: "/?clientToken=token", attempts: 2},
		{name: "POST with idempotency key", method: http.MethodPost, key: "token", attempts: 2},
		{name: "POST with empty credentials", method: http.MethodPost, path: "/?clientToken=", key: " ", attempts: 1},
		{name: "POST with empty first token", method: http.MethodPost, path: "/?clientToken=", token: "token", attempts: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			client, err := NewClientWithConfig(&Config{Endpoint: server.URL, Retry: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			request := client.R().SetRetryDelayStrategy(resty.RetryConstantDelayStrategy(time.Millisecond))
			if test.token != "" {
				request.SetQueryParam(constants.QueryClientToken, test.token)
			}
			if test.key != "" {
				request.SetHeader("Idempotency-Key", test.key)
			}
			path := test.path
			if path == "" {
				path = "/"
			}
			if _, err := request.Execute(test.method, path); err != nil {
				t.Fatal(err)
			}
			if got := attempts.Load(); got != test.attempts {
				t.Fatalf("attempts = %d, want %d", got, test.attempts)
			}
		})
	}
}
