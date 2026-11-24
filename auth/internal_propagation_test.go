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
		duplicateHeader                     string
		emptyProject                        bool
		status                              int
	}{
		{name: "no headers", caller: "service", status: 200},
		{name: "anonymous with headers", tenant: "t", user: "u", status: 401},
		{name: "ordinary user", caller: "person", tenant: "t", user: "u", status: 403},
		{name: "partial headers", caller: "service", tenant: "t", status: 403},
		{name: "project only", caller: "service", project: "project-1", status: 403},
		{name: "anonymous with empty project header", emptyProject: true, status: 403},
		{name: "duplicate tenant", caller: "service", tenant: "t", user: "u", duplicateHeader: constants.HeaderImpersonateTenantID, status: 403},
		{name: "duplicate user", caller: "service", tenant: "t", user: "u", duplicateHeader: constants.HeaderImpersonateUserID, status: 403},
		{name: "duplicate project", caller: "service", tenant: "t", user: "u", project: "p", duplicateHeader: constants.HeaderImpersonateProjectID, status: 403},
		{name: "authorized service without project", caller: "service", tenant: "tenant-1", user: "alice", status: 200},
		{name: "authorized service", caller: "service", tenant: "tenant-1", user: "alice", project: "project-1", status: 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := InternalPropagatedIdentity(func(c *core.Context) error {
				if test.duplicateHeader != "" || test.user == "" {
					t.Error("invalid propagated identity reached the authorizer")
				}
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
				engine.Use(func(c *core.Context) {
					c.Set(constants.KeyProjectID, "caller project")
					c.Set(constants.KeyUserName, "caller name")
					c.Set(constants.KeyUserRole, "caller role")
				})
			}
			engine.Use(handler)
			engine.GET("/", func(c *core.Context) {
				if test.tenant == "" {
					if UserID(c) != test.caller ||
						c.GetString(constants.KeyUserName) != "caller name" ||
						c.GetString(constants.KeyUserRole) != "caller role" {
						t.Fatal("caller identity changed without propagation headers")
					}
					c.Status(http.StatusOK)
					return
				}
				if UserID(c) != test.user || c.GetString(constants.KeyTenantID) != test.tenant ||
					c.GetString(constants.KeyProjectID) != test.project ||
					c.GetString(
						constants.KeyUserName,
					) != "" || c.GetString(constants.KeyUserRole) != "" ||
					c.GetString(constants.KeyImpersonateOriginUserID) != test.user ||
					c.GetString(constants.KeyImpersonateOriginTenantID) != test.tenant ||
					c.GetString(constants.KeyImpersonateOriginProjectID) != test.project ||
					c.GetString(constants.KeyAuthMode) != "Internal" {
					t.Fatal("propagated identity was not installed")
				}
				requireRequestIdentity(t, c, Identity{
					TenantID:        test.tenant,
					UserID:          test.user,
					ProjectID:       test.project,
					AuthMode:        "Internal",
					OriginTenantID:  test.tenant,
					OriginUserID:    test.user,
					OriginProjectID: test.project,
				})
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.caller != "" {
				req.Header.Set("Authorization", "Bearer caller-token")
			}
			req.Header.Set(constants.HeaderImpersonateTenantID, test.tenant)
			req.Header.Set(constants.HeaderImpersonateUserID, test.user)
			if test.project != "" || test.emptyProject {
				req.Header.Set(constants.HeaderImpersonateProjectID, test.project)
			}
			if test.duplicateHeader != "" {
				req.Header.Add(test.duplicateHeader, "another-value")
			}
			writer := httptest.NewRecorder()
			engine.ServeHTTP(writer, req)
			if writer.Code != test.status {
				t.Fatalf("status = %d, want %d", writer.Code, test.status)
			}
		})
	}
}
