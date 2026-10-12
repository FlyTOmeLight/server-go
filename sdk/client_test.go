package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"resty.dev/v3"
)

func TestClonePreservesSettingsAndResourceOwnership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("X-Default") != "default" || r.Header.Get("X-Signature") != "signed" || r.URL.Query().Get("key") != "value" {
			t.Errorf("clone lost request settings: headers=%v query=%v", r.Header, r.URL.Query())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	config := &Config{Endpoint: server.URL, Timeout: 3, Retry: 2, Credentials: &Credentials{AuthType: AuthTypeAPIKey, APIKey: "token"}}
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
		{name: "empty API key", config: &Config{Endpoint: "http://localhost", Credentials: &Credentials{AuthType: AuthTypeAPIKey}}},
		{name: "AKSK", config: &Config{Endpoint: "http://localhost", Credentials: &Credentials{AuthType: AuthTypeAKSK, AccessKeyId: "ak", SecretAccessKey: "sk"}}, unsupported: true},
		{name: "unknown auth", config: &Config{Endpoint: "http://localhost", Credentials: &Credentials{AuthType: "unknown"}}, unsupported: true},
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
