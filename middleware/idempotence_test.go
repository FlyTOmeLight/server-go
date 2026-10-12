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
)

func identityMiddleware(c *gin.Context) {
	c.Set(constants.KeyOrgID, c.GetHeader("Test-Org"))
	c.Set(constants.KeyUserID, c.GetHeader("Test-User"))
	c.Next()
}

func idempotentRequest(path, org, user, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Test-Org", org)
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
	authorize := func(c *gin.Context) {
		authorizations++
		if c.GetHeader("Revoked") != "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
	handler := func(c *gin.Context) {
		executions++
		c.JSON(
			http.StatusCreated,
			gin.H{"org": c.GetString(constants.KeyOrgID), "owner": c.GetString(constants.KeyUserID)},
		)
	}
	engine.POST("/create", authorize, Idempotent(controller, handler))
	engine.POST("/another", authorize, Idempotent(controller, handler))
	for _, tc := range []struct {
		name, path, org, user, body string
		revoked                     bool
		status, executions          int
	}{
		{"original", "/create?clientToken=token", "org1", "alice", `{}`, false, 201, 1},
		{"replay", "/create?clientToken=token", "org1", "alice", `{}`, false, 201, 1},
		{"revoked", "/create?clientToken=token", "org1", "alice", `{}`, true, 403, 1},
		{"other user", "/create?clientToken=token", "org1", "bob", `{}`, false, 201, 2},
		{"other org", "/create?clientToken=token", "org2", "alice", `{}`, false, 201, 3},
		{"other operation", "/another?clientToken=token", "org1", "alice", `{}`, false, 201, 4},
		{"body conflict", "/create?clientToken=token", "org1", "alice", `{"x":1}`, false, 409, 4},
		{"query conflict", "/create?clientToken=token&x=1", "org1", "alice", `{}`, false, 409, 4},
		{"anonymous first", "/create?clientToken=token", "", "", `{}`, false, 201, 5},
		{"anonymous repeat", "/create?clientToken=token", "", "", `{}`, false, 201, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := idempotentRequest(tc.path, tc.org, tc.user, tc.body)
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
		handler gin.HandlerFunc
		status  int
		body    string
	}{
		{"write string", func(c *gin.Context) {
			c.Header("Content-Type", "text/plain")
			c.Header("Location", "/created/1")
			c.Writer.Header().Add("Set-Cookie", "first=1")
			c.Writer.Header().Add("Set-Cookie", "second=2")
			c.Status(http.StatusCreated)
			_, _ = c.Writer.WriteString("created")
		}, 201, "created"},
		{"rendered error", func(c *gin.Context) {
			c.Abort()
			_ = c.Error(errors.NewError(http.StatusUnprocessableEntity, "InvalidInput", "invalid input"))
		}, 422, "invalid input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := idempotence.New(time.Minute)
			defer controller.Close()
			engine := gin.New()
			engine.Use(identityMiddleware, gin.Recovery(), errors.ErrorHandler(nil))
			executions := 0
			engine.POST(
				"/create",
				Idempotent(controller, func(c *gin.Context) { executions++; tc.handler(c) }),
			)
			var original *httptest.ResponseRecorder
			for range 2 {
				w := httptest.NewRecorder()
				engine.ServeHTTP(
					w,
					idempotentRequest("/create?clientToken=token", "org", "alice", `{}`),
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
	engine.POST("/create", func(c *gin.Context) {
		executions.Add(1)
		close(entered)
		<-release
		c.String(http.StatusCreated, "created")
	})
	original := httptest.NewRecorder()
	go func() {
		defer close(finished)
		engine.ServeHTTP(original, idempotentRequest("/create?clientToken=token", "org", "alice", `{}`))
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
		req := idempotentRequest("/create?clientToken=token", "org", "alice", `{}`).WithContext(ctx)
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
	engine.POST("/create", func(c *gin.Context) { t.Error("invalid body reached handler") })
	req := idempotentRequest("/create?clientToken=token", "org", "alice", `{}`)
	req.Body = failedBody{}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}
