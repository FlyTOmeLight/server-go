package internalsdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/sdk"
	"resty.dev/v3"
)

func TestWrapPropagatesContextAndKeepsRequestsIsolated(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("X-Signature") != "signed" || r.Header.Get("X-Default") != "default" {
			t.Errorf("request lost client settings: %v", r.Header)
		}
		if user := r.Header.Get(constants.HeaderImpersonateUserID); user != r.URL.Query().Get("user") {
			t.Errorf("identity mixed between requests: %v", r.Header)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := NewInternalClientWithConfig(&InternalClientConfig{Config: &sdk.Config{Endpoint: server.URL}, EnableImpersonate: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	client.SetHeader("X-Default", "default")
	client.AddRequestMiddleware(func(_ *resty.Client, request *resty.Request) error {
		request.SetHeader("X-Signature", "signed")
		return nil
	})
	inbound, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx := &core.Context{Request: httptest.NewRequest(http.MethodGet, "/", nil).WithContext(inbound)}
	request := Wrap(client, ctx)
	if request.Context() != inbound {
		t.Fatal("wrapped request does not use inbound context")
	}
	cancel()
	if _, err := request.Get("/"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if requests.Load() != 0 {
		t.Fatal("canceled request reached downstream server")
	}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			user := fmt.Sprint(i)
			ctx := &core.Context{Request: httptest.NewRequest(http.MethodGet, "/", nil)}
			ctx.Set(constants.KeyUserID, user)
			ctx.Set(constants.KeyOrgID, "organization")
			if _, err := client.R(ctx).SetQueryParam("user", user).Get("/"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if requests.Load() != 20 {
		t.Fatalf("expected 20 requests, got %d", requests.Load())
	}
	if client.Header().Get(constants.HeaderImpersonateUserID) != "" {
		t.Fatal("request identity leaked into shared client headers")
	}
}

func TestNewInternalClientRejectsUnsupportedModes(t *testing.T) {
	for _, test := range []struct {
		name   string
		config *InternalClientConfig
	}{
		{name: "nil"},
		{name: "missing base config", config: &InternalClientConfig{}},
		{name: "STS", config: &InternalClientConfig{Config: &sdk.Config{Endpoint: "http://localhost"}, EnableSTS: true}},
		{name: "idempotence", config: &InternalClientConfig{Config: &sdk.Config{Endpoint: "http://localhost"}, EnableIdempotence: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewInternalClientWithConfig(test.config)
			if err == nil || client != nil {
				t.Fatalf("got client=%v err=%v", client, err)
			}
		})
	}
	if _, err := NewInternalClient("ak", "sk", "http://localhost"); !errors.Is(err, sdk.ErrUnsupportedAuth) {
		t.Fatalf("expected unsupported AKSK error, got %v", err)
	}
}
