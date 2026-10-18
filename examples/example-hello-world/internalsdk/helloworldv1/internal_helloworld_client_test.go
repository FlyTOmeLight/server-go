package helloworldv1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/internalsdk"
	"github.com/FlyTOmeLight/server-go/sdk"
	"github.com/gin-gonic/gin"
)

func TestInternalClientConstructor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy/helloworld/internal/check" {
			t.Errorf("request path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	for _, suffix := range []string{"/proxy", "/proxy/"} {
		client, err := NewClient(server.URL + suffix)
		if err != nil {
			t.Fatal(err)
		}
		inbound, _ := gin.CreateTestContext(httptest.NewRecorder())
		inbound.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(t.Context())
		response, err := Wrap(client, inbound).Get("/check")
		completion := sdk.Complete(response, err)
		closeError := client.Close()
		if completion != nil {
			t.Fatal(completion)
		}
		if closeError != nil {
			t.Fatal(closeError)
		}
	}
	for _, endpoint := range []string{"", "relative", "ftp://localhost"} {
		if client, err := NewClient(endpoint); err == nil {
			_ = client.Close()
			t.Fatalf("invalid endpoint accepted: %q", endpoint)
		}
	}
}

func TestInternalClientConfigPreservesAuthenticationAndImpersonation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/helloworld/internal/check" || r.Header.Get("Authorization") != "Bearer token" ||
			r.Header.Get(constants.HeaderImpersonateTenantID) != "tenant" ||
			r.Header.Get(constants.HeaderImpersonateUserID) != "user" {
			t.Errorf("request = %s headers=%v", r.URL.Path, r.Header)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	config := &internalsdk.InternalClientConfig{
		Config: &sdk.Config{
			Endpoint:    server.URL + "/",
			Credentials: &sdk.Credentials{AuthType: sdk.AuthTypeAPIKey, APIKey: "token"},
		},
		EnableImpersonate: true,
	}
	client, err := NewClientWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if config.Endpoint != server.URL+"/" {
		t.Fatalf("caller config changed: %s", config.Endpoint)
	}
	inbound, _ := gin.CreateTestContext(httptest.NewRecorder())
	inbound.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(t.Context())
	inbound.Set(constants.KeyTenantID, "tenant")
	inbound.Set(constants.KeyUserID, "user")
	response, err := Wrap(client, inbound).Get("/check")
	if err := sdk.Complete(response, err); err != nil {
		t.Fatal(err)
	}
}
