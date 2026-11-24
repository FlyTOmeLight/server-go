package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/auth"
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
		if id, ok := auth.IdentityFromContext(c.Request.Context()); !ok ||
			id != (auth.Identity{UserID: "user", TenantID: "tenant", ProjectID: "project"}) {
			t.Fatalf("request identity = %+v, found = %v", id, ok)
		}
		if _, ok := testAuthFromContext(c.Request.Context()); !ok {
			t.Fatal("authorization context was lost")
		}
		c.Status(http.StatusNoContent)
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/resource", nil))
	if !handled || response.Code != http.StatusNoContent {
		t.Fatalf("handled=%v status=%d", handled, response.Code)
	}
}

type testAuthorizationWriter struct {
	http.ResponseWriter
}

func (w *testAuthorizationWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *testAuthorizationWriter) Header() http.Header {
	h := w.ResponseWriter.Header()
	h.Set("X-Wrapped-Header", "seen")
	return h
}

func (w *testAuthorizationWriter) WriteHeader(code int) {
	w.ResponseWriter.Header().Set("X-Wrapped-Status", "seen")
	w.ResponseWriter.WriteHeader(code)
}

func (w *testAuthorizationWriter) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write([]byte(strings.ToUpper(string(data))))
}

func (w *testAuthorizationWriter) Flush() {
	w.ResponseWriter.Header().Set("X-Wrapped-Flush", "seen")
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func TestHTTPAuthorizationPreservesWrappedWriter(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "response body", status: http.StatusAccepted, body: "BODY STRING"},
		{name: "status only", status: http.StatusNoContent},
		{name: "explicit default status", status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			authorize := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(&testAuthorizationWriter{ResponseWriter: w}, r)
				})
			}
			engine := gin.New()
			engine.Use(func(c *core.Context) {
				original := c.Writer
				c.Next()
				if c.Writer != original || c.Writer.Status() != test.status {
					t.Errorf("Gin writer was not restored with status %d", test.status)
				}
			})
			engine.GET("/", HTTPAuthorization(authorize, nil), func(c *core.Context) {
				c.Header("X-Handler", "seen")
				c.Status(test.status)
				if c.Writer.Written() || c.Writer.Status() != test.status {
					t.Errorf("status was not kept pending")
				}
				if test.body != "" {
					c.Writer.Flush()
					_, _ = c.Writer.Write([]byte("body"))
					_, _ = c.Writer.WriteString(" string")
					if !c.Writer.Written() || c.Writer.Size() != len(test.body) {
						t.Errorf("writer lost the response size")
					}
				}
			})
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if response.Code != test.status || response.Body.String() != test.body {
				t.Fatalf(
					"response = %d %q, want %d %q",
					response.Code,
					response.Body,
					test.status,
					test.body,
				)
			}
			for _, name := range []string{"X-Wrapped-Header", "X-Wrapped-Status", "X-Handler"} {
				if got := response.Header().Get(name); got != "seen" {
					t.Errorf("%s = %q, want seen", name, got)
				}
			}
			if test.body != "" &&
				(!response.Flushed || response.Header().Get("X-Wrapped-Flush") != "seen") {
				t.Fatal("flush did not go through the wrapped writer")
			}
		})
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
