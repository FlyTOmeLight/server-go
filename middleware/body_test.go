package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/idempotence"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
)

type countedBody struct {
	io.Reader
	read int
}

func (b *countedBody) Read(data []byte) (int, error) {
	n, err := b.Reader.Read(data)
	b.read += n
	return n, err
}

func (b *countedBody) Close() error { return nil }

func TestJSONReadBudgetAcrossMiddleware(t *testing.T) {
	for _, setup := range []struct {
		name                             string
		contentType                      string
		debug, idempotent, noContentType bool
	}{
		{name: "binder"},
		{name: "debug", debug: true},
		{name: "idempotent", idempotent: true},
		{name: "debug and idempotent", debug: true, idempotent: true},
		{name: "binder without content type", noContentType: true},
		{name: "debug without content type", debug: true, noContentType: true},
		{name: "idempotent without content type", idempotent: true, noContentType: true},
		{name: "debug and idempotent without content type", debug: true, idempotent: true, noContentType: true},
		{name: "binder with invalid JSON parameters", contentType: "application/json;broken"},
		{name: "debug with invalid JSON parameters", debug: true, contentType: "application/json;broken"},
		{name: "idempotent with invalid JSON parameters", idempotent: true, contentType: "application/json;broken"},
		{name: "debug and idempotent with invalid JSON parameters", debug: true, idempotent: true, contentType: "application/json;broken"},
	} {
		t.Run(setup.name, func(t *testing.T) {
			for _, oversized := range []bool{false, true} {
				for _, unknownLength := range []bool{false, true} {
					payload := `{"name":"ok"}`
					if oversized {
						payload = `{"name":"` + strings.Repeat(
							"x",
							binder.MaxJSONBytes*2,
						) + `"}`
					}
					engine := gin.New()
					engine.Use(errors.ErrorHandler(nil), identityMiddleware)
					if setup.debug {
						engine.Use(Debug(nil, nil))
					}
					executions := 0
					handler := func(c *core.Context) {
						var req struct {
							Name string `json:"name" binding:"required"`
						}
						if !binder.BindJSON(c, &req) {
							return
						}
						executions++
						c.String(http.StatusOK, req.Name)
					}
					controller := idempotence.New(time.Minute)
					if setup.idempotent {
						handler = Idempotent(controller, handler)
					}
					engine.POST("/create", handler)
					request := idempotentRequest(
						"/create?clientToken=token",
						"org",
						"user",
						payload,
					)
					body := &countedBody{Reader: strings.NewReader(payload)}
					if setup.noContentType {
						request.Header.Del("Content-Type")
					}
					if setup.contentType != "" {
						request.Header.Set("Content-Type", setup.contentType)
					}
					request.Body = body
					if unknownLength {
						request.ContentLength = -1
					}
					writer := httptest.NewRecorder()
					engine.ServeHTTP(writer, request)
					if err := controller.Close(); err != nil {
						t.Fatal(err)
					}
					if oversized {
						if writer.Code != 413 ||
							!strings.Contains(
								writer.Body.String(),
								`"code":"PayloadTooLarge"`,
							) ||
							executions != 0 ||
							body.read > binder.MaxJSONBytes+1 {
							t.Fatalf(
								"oversized request: status=%d executions=%d read=%d body=%s",
								writer.Code,
								executions,
								body.read,
								writer.Body,
							)
						}
					} else if writer.Code != 200 || writer.Body.String() != "ok" || executions != 1 || body.read != len(payload) {
						t.Fatalf(
							"valid request: status=%d executions=%d read=%d body=%s",
							writer.Code,
							executions,
							body.read,
							writer.Body,
						)
					}
				}
			}
		})
	}
}

func TestAuto200ImmediateErrorsAreNotCached(t *testing.T) {
	controller := idempotence.New(time.Minute)
	defer controller.Close()
	engine := gin.New()
	engine.Use(errors.ErrorHandler(&conf.Config{
		AppName: "example", AppCode: 731,
		HTTPServer: conf.HTTPServerConfig{AltoHandlerAPIError: true},
	}), identityMiddleware)
	executions := 0
	engine.POST("/example/api/create", Idempotent(controller, func(c *core.Context) {
		executions++
		errors.Abort(c, http.StatusUnprocessableEntity, "InvalidInput", "invalid input")
	}))
	for range 2 {
		writer := httptest.NewRecorder()
		engine.ServeHTTP(
			writer,
			idempotentRequest("/example/api/create?clientToken=token", "org", "user", `{}`),
		)
		if writer.Code != 200 || !strings.Contains(writer.Body.String(), `"statusCode":731422`) {
			t.Fatalf("failure contract: %d %s", writer.Code, writer.Body)
		}
	}
	if executions != 2 {
		t.Fatalf("failed operation executed %d times", executions)
	}
}

func TestJSONReadFailureAcrossMiddleware(t *testing.T) {
	for _, setup := range []struct {
		name              string
		debug, idempotent bool
	}{
		{name: "binder"},
		{name: "debug", debug: true},
		{name: "idempotent", idempotent: true},
		{name: "debug and idempotent", debug: true, idempotent: true},
	} {
		t.Run(setup.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(errors.ErrorHandler(nil), identityMiddleware)
			if setup.debug {
				engine.Use(Debug(nil, nil))
			}
			executed := false
			handler := func(c *core.Context) {
				var req struct {
					Name string `json:"name"`
				}
				if binder.BindJSON(c, &req) {
					executed = true
				}
			}
			controller := idempotence.New(time.Minute)
			defer controller.Close()
			if setup.idempotent {
				handler = Idempotent(controller, handler)
			}
			engine.POST("/create", handler)
			request := idempotentRequest("/create?clientToken=token", "org", "user", `{}`)
			request.Body = failedBody{}
			request.ContentLength = -1
			writer := httptest.NewRecorder()
			engine.ServeHTTP(writer, request)
			code := "InvalidParameter"
			if setup.debug || setup.idempotent {
				code = "InvalidRequestBody"
			}
			if writer.Code != http.StatusBadRequest || executed ||
				!strings.Contains(writer.Body.String(), `"code":"`+code+`"`) {
				t.Fatalf(
					"failed body: status=%d executed=%v body=%s",
					writer.Code,
					executed,
					writer.Body,
				)
			}
		})
	}
}
