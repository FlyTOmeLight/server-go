package helloworldv1

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/sdk"
)

func TestModuleSDKHTTPFailures(t *testing.T) {
	for _, test := range []struct{ name, contentType, body, code string }{
		{name: "service error", contentType: "application/json", body: `{"code":"Forbidden","message":"denied","requestId":"request-1"}`, code: "Forbidden"},
		{name: "HTML", contentType: "text/html", body: "<html>gateway error</html>", code: "HTTPError"},
		{name: "invalid JSON", contentType: "application/json", body: "{", code: "HTTPError"},
		{name: "empty object", contentType: "application/json", body: "{}", code: "HTTPError"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := NewClientWithConfig(&sdk.Config{Endpoint: server.URL, Timeout: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			for _, call := range []func() error{
				func() error { _, err := client.CreateModule(context.Background(), &CreateModuleRequest{}); return err },
				func() error { _, err := client.ListModule(context.Background(), &ListModuleRequest{}); return err },
				func() error { _, err := client.GetModule(context.Background(), &GetModuleRequest{}); return err },
				func() error { return client.UpdateModule(context.Background(), &UpdateModuleRequest{}) },
				func() error { return client.DeleteModule(context.Background(), &DeleteModuleRequest{}) },
			} {
				var detail *serrors.Error
				if err := call(); !errors.As(err, &detail) || detail.Status != http.StatusForbidden ||
					detail.Code != test.code {
					t.Fatalf("incorrect HTTP error: %#v", err)
				}
				if test.code == "Forbidden" && detail.RequestID != "request-1" {
					t.Fatal("request ID lost")
				}
			}
		})
	}
}

func TestModuleSDKCancellation(t *testing.T) {
	entered := make(chan struct{})
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
		close(stopped)
	}))
	defer server.Close()
	client, err := NewClientWithConfig(&sdk.Config{Endpoint: server.URL, Timeout: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := client.GetModule(ctx, &GetModuleRequest{}); done <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request did not cancel")
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("server context did not cancel")
	}
}
