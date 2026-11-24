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
	c.Set(constants.KeyProjectID, c.GetHeader("Test-Project"))
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
					if tc.status < 400 && len(w.Header().Values("Set-Cookie")) != 2 {
						t.Fatalf("original response lost cookies: %v", w.Header())
					}
					original = w
					continue
				}
				if w.Body.String() != original.Body.String() ||
					w.Header().Get("Content-Type") != original.Header().Get("Content-Type") ||
					w.Header().Get("Location") != original.Header().Get("Location") ||
					len(w.Header().Values("Set-Cookie")) != 0 {
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

func TestIdempotenceWithStoreProjectAndHeaderScopes(t *testing.T) {
	controller := idempotence.New(time.Minute)
	defer controller.Close()
	engine := gin.New()
	engine.Use(identityMiddleware, IdempotenceWithStore(controller))
	executions := 0
	engine.POST("/create", func(c *core.Context) {
		executions++
		c.String(http.StatusCreated, "execution %d", executions)
	})
	for _, tc := range []struct {
		path, token, user, project string
		wantBody                   string
	}{
		{path: "/create", token: " token ", user: "bearer", project: "project-a", wantBody: "execution 1"},
		{path: "/create", token: "token", user: "bearer", project: "project-a", wantBody: "execution 1"},
		{path: "/create", token: "token", user: "bearer", project: "project-b", wantBody: "execution 2"},
		{path: "/create", token: "token", user: "", project: "project-a", wantBody: "execution 3"},
		{path: "/create", token: "token", user: "", project: "project-a", wantBody: "execution 4"},
		{path: "/create?clientToken=query", token: "first", user: "bearer", project: "", wantBody: "execution 5"},
		{path: "/create?clientToken=query", token: "second", user: "bearer", project: "", wantBody: "execution 5"},
	} {
		request := idempotentRequest(tc.path, "", tc.user, `{}`)
		request.Header.Set("Idempotency-Key", tc.token)
		request.Header.Set("Test-Project", tc.project)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusCreated || recorder.Body.String() != tc.wantBody {
			t.Errorf("unexpected result for %+v: %d %s", tc, recorder.Code, recorder.Body)
		}
	}
}

func TestIdempotenceReplayKeepsCurrentRequestHeaders(t *testing.T) {
	controller := idempotence.New(time.Minute)
	defer controller.Close()
	engine := gin.New()
	engine.Use(identityMiddleware, RequestID(), CORS(CORSConfig{
		AllowOrigins: []string{"https://first.example", "https://second.example"},
	}))
	var outerWriter http.ResponseWriter
	var executions int
	engine.POST("/create", func(c *core.Context) {
		outerWriter = c.Writer
		c.Next()
	}, IdempotentWithStore(controller, func(c *core.Context) {
		executions++
		if c.Writer.(interface{ Unwrap() http.ResponseWriter }).Unwrap() != outerWriter {
			t.Error("writer did not unwrap to the original response writer")
		}
		c.Header("Date", "Mon, 01 Jan 2024 00:00:00 GMT")
		c.Header("Set-Cookie", "session=first")
		c.Writer.Header().Add("Vary", "Accept-Encoding")
		c.String(http.StatusCreated, "created")
	}))
	for _, tc := range []struct {
		id, origin string
		replay     bool
	}{
		{id: "first", origin: "https://first.example", replay: false},
		{id: "second", origin: "https://second.example", replay: true},
		{id: "third", origin: "", replay: true},
	} {
		request := idempotentRequest("/create?clientToken=token", "tenant", "alice", `{}`)
		request.Header.Set(constants.HeaderRequestID, tc.id)
		request.Header.Set("Origin", tc.origin)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		header := recorder.Result().Header
		if recorder.Code != http.StatusCreated || recorder.Body.String() != "created" ||
			header.Get(
				constants.HeaderRequestID,
			) != tc.id || header.Get("Access-Control-Allow-Origin") != tc.origin ||
			strings.Join(header.Values("Vary"), ", ") != "Origin, Accept-Encoding" {
			t.Fatalf("unexpected result for %s: %d %s %v", tc.id, recorder.Code, recorder.Body, header)
		}
		if tc.replay && (header.Get("Date") != "" || header.Get("Set-Cookie") != "") {
			t.Fatalf("replay included stale headers: %v", header)
		}
	}
	if executions != 1 {
		t.Fatalf("handler executed %d times, want one", executions)
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
