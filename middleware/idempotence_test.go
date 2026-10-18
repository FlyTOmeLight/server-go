package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/idempotence"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func identityMiddleware(c *core.Context) {
	c.Set(constants.KeyTenantID, c.GetHeader("Test-Tenant"))
	c.Set(constants.KeyUserID, c.GetHeader("Test-User"))
	c.Next()
}

func idempotentRequest(path, tenant, user, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Test-Tenant", tenant)
	req.Header.Set("Test-User", user)
	return req
}

func TestIdempotenceAuthorizationAndIdentityScopes(t *testing.T) {
	controller := idempotence.New(time.Minute)
	defer controller.Close()
	engine := gin.New()
	authorizations := 0
	executions := 0
	engine.Use(identityMiddleware)
	authorize := func(c *core.Context) {
		authorizations++
		if c.GetHeader("Revoked") != "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
	handler := func(c *core.Context) {
		executions++
		c.JSON(
			http.StatusCreated,
			gin.H{"tenant": c.GetString(constants.KeyTenantID), "owner": c.GetString(constants.KeyUserID)},
		)
	}
	engine.POST("/create", authorize, Idempotent(controller, handler))
	engine.POST("/another", authorize, Idempotent(controller, handler))
	for _, tc := range []struct {
		name, path, tenant, user, body string
		revoked                        bool
		status, executions             int
	}{
		{name: "original", path: "/create?clientToken=token", tenant: "tenant1", user: "alice", body: `{}`, revoked: false, status: 201, executions: 1},
		{name: "replay", path: "/create?clientToken=token", tenant: "tenant1", user: "alice", body: `{}`, revoked: false, status: 201, executions: 1},
		{name: "revoked", path: "/create?clientToken=token", tenant: "tenant1", user: "alice", body: `{}`, revoked: true, status: 403, executions: 1},
		{name: "other user", path: "/create?clientToken=token", tenant: "tenant1", user: "bob", body: `{}`, revoked: false, status: 201, executions: 2},
		{name: "other tenant", path: "/create?clientToken=token", tenant: "tenant2", user: "alice", body: `{}`, revoked: false, status: 201, executions: 3},
		{name: "other operation", path: "/another?clientToken=token", tenant: "tenant1", user: "alice", body: `{}`, revoked: false, status: 201, executions: 4},
		{name: "body conflict", path: "/create?clientToken=token", tenant: "tenant1", user: "alice", body: `{"x":1}`, revoked: false, status: 409, executions: 4},
		{name: "query conflict", path: "/create?clientToken=token&x=1", tenant: "tenant1", user: "alice", body: `{}`, revoked: false, status: 409, executions: 4},
		{name: "anonymous first", path: "/create?clientToken=token", tenant: "", user: "", body: `{}`, revoked: false, status: 201, executions: 5},
		{name: "anonymous repeat", path: "/create?clientToken=token", tenant: "", user: "", body: `{}`, revoked: false, status: 201, executions: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := idempotentRequest(tc.path, tc.tenant, tc.user, tc.body)
			if tc.revoked {
				req.Header.Set("Revoked", "true")
			}
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if w.Code != tc.status || executions != tc.executions {
				t.Fatalf("status=%d executions=%d body=%s", w.Code, executions, w.Body.String())
			}
			if tc.status == 201 && !strings.Contains(w.Body.String(), `"owner":"`+tc.user+`"`) {
				t.Fatalf("wrong owner: %s", w.Body.String())
			}
		})
	}
	if authorizations != 10 {
		t.Fatalf("authorization executed %d times", authorizations)
	}
}

func TestIdempotenceReplayCompleteResponseAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler core.HandlerFunc
		status  int
		body    string
	}{
		{name: "write string", handler: func(c *core.Context) {
			c.Header("Content-Type", "text/plain")
			c.Header("Location", "/created/1")
			c.Writer.Header().Add("Set-Cookie", "first=1")
			c.Writer.Header().Add("Set-Cookie", "second=2")
			c.Status(http.StatusCreated)
			_, _ = c.Writer.WriteString("created")
		}, status: 201, body: "created"},
		{name: "rendered error", handler: func(c *core.Context) {
			c.Abort()
			_ = c.Error(errors.NewError(http.StatusUnprocessableEntity, "InvalidInput", "invalid input"))
		}, status: 422, body: "invalid input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := idempotence.New(time.Minute)
			defer controller.Close()
			engine := gin.New()
			engine.Use(identityMiddleware, gin.Recovery(), errors.ErrorHandler(nil))
			executions := 0
			engine.POST(
				"/create",
				Idempotent(controller, func(c *core.Context) { executions++; tc.handler(c) }),
			)
			var original *httptest.ResponseRecorder
			for range 2 {
				w := httptest.NewRecorder()
				engine.ServeHTTP(
					w,
					idempotentRequest("/create?clientToken=token", "tenant", "alice", `{}`),
				)
				if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.body) {
					t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
				}
				if original == nil {
					original = w
					continue
				}
				if w.Body.String() != original.Body.String() ||
					w.Header().Get("Content-Type") != original.Header().Get("Content-Type") ||
					w.Header().Get("Location") != original.Header().Get("Location") ||
					strings.Join(
						w.Header().Values("Set-Cookie"),
						";",
					) != strings.Join(
						original.Header().Values("Set-Cookie"),
						";",
					) {
					t.Fatalf("incomplete replay: headers=%v body=%s", w.Header(), w.Body.String())
				}
			}
			wantExecutions := 1
			if tc.status >= 400 {
				wantExecutions = 2
			}
			if executions != wantExecutions {
				t.Fatalf("executed %d times, want %d", executions, wantExecutions)
			}
		})
	}
}

func TestIdempotenceConcurrentWaitCancellation(t *testing.T) {
	controller := idempotence.New(time.Minute)
	defer controller.Close()
	engine := gin.New()
	engine.Use(identityMiddleware, Idempotence(controller))
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	var executions atomic.Int32
	engine.POST("/create", func(c *core.Context) {
		executions.Add(1)
		close(entered)
		<-release
		c.String(http.StatusCreated, "created")
	})
	original := httptest.NewRecorder()
	go func() {
		defer close(finished)
		engine.ServeHTTP(original, idempotentRequest("/create?clientToken=token", "tenant", "alice", `{}`))
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("operation did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	waiterDone := make(chan struct{})
	waiter := httptest.NewRecorder()
	go func() {
		defer close(waiterDone)
		req := idempotentRequest("/create?clientToken=token", "tenant", "alice", `{}`).WithContext(ctx)
		engine.ServeHTTP(waiter, req)
	}()
	cancel()
	select {
	case <-waiterDone:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("waiter did not return")
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("original request did not complete")
	}
	if waiter.Code != http.StatusRequestTimeout || original.Code != http.StatusCreated || executions.Load() != 1 {
		t.Fatalf("waiter=%d original=%d executions=%d", waiter.Code, original.Code, executions.Load())
	}
}

type failedBody struct{}

func (failedBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func (failedBody) Close() error { return nil }

func TestIdempotenceBodyReadFailureDoesNotExecute(t *testing.T) {
	controller := idempotence.New(time.Minute)
	defer controller.Close()
	engine := gin.New()
	engine.Use(identityMiddleware, Idempotence(controller))
	engine.POST("/create", func(c *core.Context) { t.Error("invalid body reached handler") })
	req := idempotentRequest("/create?clientToken=token", "tenant", "alice", `{}`)
	req.Body = failedBody{}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}
