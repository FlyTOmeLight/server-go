package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestTrustedIdentityHeaders(t *testing.T) {
	if _, err := TrustedIdentityHeaders(nil); err == nil {
		t.Fatal("missing gateway verifier accepted")
	}
	verify := func(r *http.Request) error {
		if r.Header.Get("Authorization") != "Bearer gateway-secret" {
			return errors.New("not a trusted gateway")
		}
		return nil
	}
	for _, test := range []struct {
		name, token, tenant, user, project string
		duplicateHeader                    string
		prior                              core.HandlerFunc
		status                             int
	}{
		{name: "forged identity", tenant: "target-tenant", user: "target-user", status: 401},
		{name: "missing identity", token: "gateway-secret", status: 401},
		{name: "partial identity", token: "gateway-secret", user: "target-user", status: 401},
		{name: "trusted gateway", token: "gateway-secret", tenant: "target-tenant", user: "target-user", status: 200},
		{name: "trusted gateway with project", token: "gateway-secret", tenant: "target-tenant", user: "target-user", project: "target-project", status: 200},
		{name: "duplicate tenant", token: "gateway-secret", tenant: "target-tenant", user: "target-user", duplicateHeader: constants.HeaderTenantID, status: 401},
		{name: "duplicate user", token: "gateway-secret", tenant: "target-tenant", user: "target-user", duplicateHeader: constants.HeaderUserID, status: 401},
		{name: "duplicate project", token: "gateway-secret", tenant: "target-tenant", user: "target-user", project: "target-project", duplicateHeader: constants.HeaderProjectID, status: 401},
		{name: "cannot replace bearer identity", token: "gateway-secret", tenant: "target-tenant", user: "target-user", prior: BearerTokens(map[string]string{"gateway-secret": "caller"}), status: 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := TrustedIdentityHeaders(verify)
			if err != nil {
				t.Fatal(err)
			}
			engine := gin.New()
			engine.Use(func(c *core.Context) {
				c.Set(constants.KeyUserName, "previous name")
				c.Set(constants.KeyUserRole, "previous role")
			})
			if test.prior != nil {
				engine.Use(test.prior)
			}
			engine.Use(handler)
			engine.GET("/", func(c *core.Context) {
				if c.GetString(constants.KeyTenantID) != test.tenant || UserID(c) != test.user ||
					c.GetString(constants.KeyProjectID) != test.project {
					t.Fatal("trusted identity was not installed")
				}
				if c.GetString(constants.KeyUserName) != "" ||
					c.GetString(constants.KeyUserRole) != "" {
					t.Fatal("trusted identity inherited stale user metadata")
				}
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.token != "" {
				req.Header.Set("Authorization", "Bearer "+test.token)
			}
			req.Header.Set(constants.HeaderTenantID, test.tenant)
			req.Header.Set(constants.HeaderUserID, test.user)
			req.Header.Set(constants.HeaderProjectID, test.project)
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

func TestUseRejectsUnverifiedAuthCenter(t *testing.T) {
	engine := gin.New()
	if err := Use(
		engine,
		&conf.Config{HTTPServer: conf.HTTPServerConfig{AuthType: conf.AuthTypeAuthCenter}},
	); err == nil {
		t.Fatal("unverified AuthCenter accepted")
	}
}

func TestImpersonateRequiresAuthorizedServiceAccount(t *testing.T) {
	if _, err := Impersonate(nil); err == nil {
		t.Fatal("missing service account authorizer accepted")
	}
	for _, test := range []struct {
		name, caller, tenant, user, project string
		expectedProject, duplicateHeader    string
		emptyProject                        bool
		status                              int
	}{
		{name: "anonymous", tenant: "target-tenant", user: "target-user", status: 401},
		{name: "ordinary user", caller: "person", tenant: "target-tenant", user: "target-user", status: 403},
		{name: "partial target", caller: "service", tenant: "target-tenant", status: 403},
		{name: "project only", caller: "service", project: "target-project", status: 403},
		{name: "anonymous with empty project header", emptyProject: true, status: 403},
		{name: "duplicate tenant", caller: "service", tenant: "target-tenant", user: "target-user", duplicateHeader: constants.HeaderImpersonateTenantID, status: 403},
		{name: "duplicate user", caller: "service", tenant: "target-tenant", user: "target-user", duplicateHeader: constants.HeaderImpersonateUserID, status: 403},
		{name: "duplicate project", caller: "service", tenant: "target-tenant", user: "target-user", project: "target-project", duplicateHeader: constants.HeaderImpersonateProjectID, status: 403},
		{name: "unauthorized target", caller: "service", tenant: "other-tenant", user: "target-user", status: 403},
		{name: "authorized service without project", caller: "service", tenant: "target-tenant", user: "target-user", expectedProject: "origin-project", status: 200},
		{name: "authorized service with project", caller: "service", tenant: "target-tenant", user: "target-user", project: "target-project", expectedProject: "target-project", status: 200},
		{name: "no impersonation", caller: "person", status: 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := Impersonate(func(c *core.Context, tenantID, userID string) error {
				if test.duplicateHeader != "" || test.user == "" {
					t.Error("invalid impersonation reached the authorizer")
				}
				if UserID(c) != "service" || tenantID != "target-tenant" || userID != "target-user" {
					return errors.New("caller may not impersonate this target")
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
					c.Set(constants.KeyProjectID, "origin-project")
					c.Set(constants.KeyUserName, "caller name")
					c.Set(constants.KeyUserRole, "caller role")
				})
			}
			engine.Use(handler)
			engine.GET("/", func(c *core.Context) {
				if test.user == "" {
					if UserID(c) != test.caller ||
						c.GetString(constants.KeyUserName) != "caller name" ||
						c.GetString(constants.KeyUserRole) != "caller role" {
						t.Fatal("caller identity changed without impersonation")
					}
				} else if UserID(c) != test.user || c.GetString(constants.KeyTenantID) != test.tenant ||
					c.GetString(constants.KeyProjectID) != test.expectedProject ||
					c.GetString(constants.KeyImpersonateOriginUserID) != test.caller ||
					c.GetString(constants.KeyImpersonateOriginProjectID) != "origin-project" {
					t.Fatal("target or original identity was lost")
				}
				if test.user != "" &&
					(c.GetString(constants.KeyUserName) != "" || c.GetString(constants.KeyUserRole) != "") {
					t.Fatal("impersonated identity inherited caller metadata")
				}
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
