package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type testAuthKey struct{}

type testAuth struct {
	UserID    string
	TenantID  string
	ProjectID string
}

func withTestAuth(ctx context.Context, auth testAuth) context.Context {
	return context.WithValue(ctx, testAuthKey{}, auth)
}

func testAuthFromContext(ctx context.Context) (testAuth, bool) {
	auth, _ := ctx.Value(testAuthKey{}).(testAuth)
	return auth, auth.UserID != ""
}

func TestHTTPAuthorizationPublishesVerifiedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authorize := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := withTestAuth(r.Context(), testAuth{
				UserID: "user", TenantID: "tenant", ProjectID: "project",
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	publish := func(c *core.Context, r *http.Request) {
		auth, ok := testAuthFromContext(r.Context())
		if !ok {
			t.Fatal("missing auth")
		}
		c.Set(constants.KeyUserID, auth.UserID)
		c.Set(constants.KeyTenantID, auth.TenantID)
		c.Set(constants.KeyProjectID, auth.ProjectID)
	}

	handled := false
	engine := gin.New()
	engine.GET("/resource", HTTPAuthorization(authorize, publish), func(c *core.Context) {
		handled = true
		if c.GetString(constants.KeyUserID) != "user" ||
			c.GetString(constants.KeyTenantID) != "tenant" ||
			c.GetString(constants.KeyProjectID) != "project" {
			t.Fatalf("keys user=%q tenant=%q project=%q",
				c.GetString(constants.KeyUserID),
				c.GetString(constants.KeyTenantID),
				c.GetString(constants.KeyProjectID),
			)
		}
		c.Status(http.StatusNoContent)
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/resource", nil))
	if !handled || response.Code != http.StatusNoContent {
		t.Fatalf("handled=%v status=%d", handled, response.Code)
	}
}

func TestHTTPAuthorizationAbortsRejectedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authorize := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
	handled := false
	engine := gin.New()
	engine.GET("/resource", HTTPAuthorization(authorize, nil), func(c *core.Context) {
		handled = true
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/resource", nil))
	if handled || response.Code != http.StatusUnauthorized {
		t.Fatalf("handled=%v status=%d", handled, response.Code)
	}
}
