package internalsdk_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/auth"
	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/internalsdk"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/sdk"
)

const (
	serviceToken = "agentic-kit-service-token-0001"
	personToken  = "ordinary-person-token-000001"
)

// TestInternalPropagationReachesDownstream shows agentic-kit-api forwarding a
// verified end-user identity to coding through internalsdk. Coding accepts the
// identity only from an authorized internal service and records it in both Key*
// and KeyImpersonateOrigin*.
func TestInternalPropagationReachesDownstream(t *testing.T) {
	downstream := gin.New()
	downstream.Use(auth.BearerTokens(map[string]string{
		serviceToken: "agentic-kit-api",
		personToken:  "person",
	}))
	propagate, err := auth.InternalPropagatedIdentity(func(c *core.Context) error {
		if auth.UserID(c) != "agentic-kit-api" {
			return errors.New("caller is not an internal service")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	downstream.Use(propagate)
	downstream.GET("/check", func(c *core.Context) {
		identity, ok := auth.IdentityFromContext(c.Request.Context())
		if !ok {
			t.Error("authenticated identity missing from downstream request context")
		}
		c.JSON(http.StatusOK, gin.H{
			"tenant":        identity.TenantID,
			"user":          identity.UserID,
			"project":       identity.ProjectID,
			"originUser":    identity.OriginUserID,
			"originTenant":  identity.OriginTenantID,
			"originProject": identity.OriginProjectID,
			"authMode":      identity.AuthMode,
			"requestID":     c.GetHeader(constants.HeaderRequestID),
		})
	})
	server := httptest.NewServer(downstream)
	defer server.Close()

	for _, test := range []struct {
		name, token, project string
		status               int
	}{
		{name: "internal service", token: serviceToken, project: "project-1", status: http.StatusOK},
		{name: "internal service without project", token: serviceToken, status: http.StatusOK},
		{name: "ordinary user", token: personToken, status: http.StatusForbidden},
		{name: "unknown token", token: "unknown-token-0000000000001", status: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller, err := internalsdk.NewInternalClientWithConfig(&internalsdk.InternalClientConfig{
				Config: &sdk.Config{
					Endpoint:    server.URL,
					Credentials: &sdk.Credentials{AuthType: sdk.AuthTypeAPIKey, APIKey: test.token},
				},
				EnableImpersonate: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer caller.Close()

			inbound, _ := gin.CreateTestContext(httptest.NewRecorder())
			inbound.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			inbound.Set(constants.KeyTenantID, "tenant-1")
			inbound.Set(constants.KeyUserID, "alice")
			inbound.Set(constants.KeyProjectID, test.project)
			inbound.Set(constants.KeyRequestID, "verified-request-id")
			inbound.Request.Header.Set(constants.HeaderRequestID, "raw-request-id")

			response, err := internalsdk.Wrap(caller, inbound).Get("/check")
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode() != test.status {
				t.Fatalf(
					"status = %d, want %d: %s",
					response.StatusCode(),
					test.status,
					response.Bytes(),
				)
			}
			if test.status != http.StatusOK {
				return
			}
			var got map[string]string
			if err := json.Unmarshal(response.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{
				"tenant":        "tenant-1",
				"user":          "alice",
				"project":       test.project,
				"originUser":    "alice",
				"originTenant":  "tenant-1",
				"originProject": test.project,
				"authMode":      "Internal",
				"requestID":     "verified-request-id",
			}
			for key, value := range want {
				if got[key] != value {
					t.Fatalf("%s = %q, want %q (body %v)", key, got[key], value, got)
				}
			}
		})
	}
}
