package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestInternalPropagatedIdentity(t *testing.T) {
	if _, err := InternalPropagatedIdentity(nil); err == nil {
		t.Fatal("nil authorizer accepted")
	}
	for _, test := range []struct {
		name, caller, tenant, user, project string
		status                              int
	}{
		{name: "no headers", caller: "service", status: 200},
		{name: "anonymous with headers", tenant: "t", user: "u", status: 401},
		{name: "ordinary user", caller: "person", tenant: "t", user: "u", status: 403},
		{name: "partial headers", caller: "service", tenant: "t", status: 403},
		{name: "authorized service", caller: "service", tenant: "tenant-1", user: "alice", project: "project-1", status: 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := InternalPropagatedIdentity(func(c *core.Context) error {
				if UserID(c) != "service" {
					return errors.New("not a service account")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			engine := gin.New()
			if test.caller != "" {
				engine.Use(BearerTokens(map[string]string{"caller-token": test.caller}))
			}
			engine.Use(handler)
			engine.GET("/", func(c *core.Context) {
				if test.tenant == "" {
					if UserID(c) != test.caller {
						t.Fatal("caller identity changed without propagation headers")
					}
					c.Status(http.StatusOK)
					return
				}
				if UserID(c) != test.user || c.GetString(constants.KeyTenantID) != test.tenant ||
					c.GetString(constants.KeyProjectID) != test.project ||
					c.GetString(constants.KeyImpersonateOriginUserID) != test.user ||
					c.GetString(constants.KeyImpersonateOriginTenantID) != test.tenant ||
					c.GetString(constants.KeyImpersonateOriginProjectID) != test.project ||
					c.GetString(constants.KeyAuthMode) != "Internal" {
					t.Fatal("propagated identity was not installed")
				}
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.caller != "" {
				req.Header.Set("Authorization", "Bearer caller-token")
			}
			req.Header.Set(constants.HeaderImpersonateTenantID, test.tenant)
			req.Header.Set(constants.HeaderImpersonateUserID, test.user)
			req.Header.Set(constants.HeaderImpersonateProjectID, test.project)
			writer := httptest.NewRecorder()
			engine.ServeHTTP(writer, req)
			if writer.Code != test.status {
				t.Fatalf("status = %d, want %d", writer.Code, test.status)
			}
		})
	}
}
