package binder

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/creasty/defaults"
	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type customParameter struct{ Value string }

func (value *customParameter) UnmarshalParam(input string) error {
	if !strings.HasPrefix(input, "custom:") {
		return fmt.Errorf("invalid custom parameter: %q", input)
	}
	value.Value = strings.TrimPrefix(input, "custom:")
	return nil
}

func TestAllMergesSourcesBeforeValidation(t *testing.T) {
	for _, test := range []struct {
		name, path, body string
		status           int
	}{
		{name: "complete", path: "/object?limit=5&token=custom:ok", body: `{"name":"body","value":"body"}`, status: 200},
		{name: "missing body field", path: "/object?limit=5&token=custom:ok", body: `{}`, status: 400},
		{name: "missing URI field", path: "/?limit=5&token=custom:ok", body: `{"name":"body"}`, status: 400},
		{name: "missing query field", path: "/object?token=custom:ok", body: `{"name":"body"}`, status: 400},
		{name: "illegal query", path: "/object?limit=-1&token=custom:ok", body: `{"name":"body"}`, status: 400},
		{name: "custom unmarshal error", path: "/object?limit=5&token=bad", body: `{"name":"body"}`, status: 400},
		{name: "unknown JSON", path: "/object?limit=5&token=custom:ok", body: `{"name":"body","extra":true}`, status: 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			handler := func(c *core.Context) {
				var request struct {
					Name  string          `json:"name" binding:"required"`
					ID    string          `uri:"id" binding:"required"`
					Limit int             `form:"limit" binding:"required,min=1"`
					Token customParameter `form:"token"`
					Value string          `json:"value" form:"value" uri:"id"`
				}
				if err := Bind(c, &request).All().Error(); err != nil {
					errors.AbortWithError(c, err)
					return
				}
				if request.Name != "body" || request.ID != "object" || request.Limit != 5 ||
					request.Token.Value != "ok" ||
					request.Value != "object" {
					t.Fatalf("merged request: %#v", request)
				}
				c.Status(200)
			}
			engine.POST("/", handler)
			engine.POST("/:id", handler)
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			writer := httptest.NewRecorder()
			engine.ServeHTTP(writer, request)
			if writer.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", writer.Code, test.status, writer.Body)
			}
		})
	}
}

func TestDefaultsBeforeBindingPreserveExplicitValues(t *testing.T) {
	for _, test := range []struct {
		name, body  string
		flag        bool
		count       int
		optional    int
		nilOptional bool
	}{
		{name: "omitted", body: `{}`, flag: true, count: 5, optional: 7},
		{name: "explicit zero false null", body: `{"flag":false,"count":0,"optional":null}`, nilOptional: true},
		{name: "explicit values", body: `{"flag":true,"count":9,"optional":3}`, flag: true, count: 9, optional: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST("/", func(c *core.Context) {
				var request struct {
					Flag     bool `json:"flag" default:"true"`
					Count    int  `json:"count" default:"5" binding:"min=0"`
					Optional *int `json:"optional" default:"7"`
				}
				if err := defaults.Set(&request); err != nil {
					t.Fatal(err)
				}
				if err := Bind(c, &request).All().Error(); err != nil {
					t.Fatal(err)
				}
				if request.Flag != test.flag || request.Count != test.count ||
					(request.Optional == nil) != test.nilOptional {
					t.Fatalf("explicit value overwritten: %#v", request)
				}
				if request.Optional != nil && *request.Optional != test.optional {
					t.Fatalf("optional value = %d, want %d", *request.Optional, test.optional)
				}
				c.Status(200)
			})
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(httptest.NewRecorder(), request)
		})
	}
}

func TestAllMultipartPreservesFilesAndCustomParameters(t *testing.T) {
	type nestedFiles struct {
		File *multipart.FileHeader `form:"nested"`
	}
	type embeddedFiles struct {
		Embedded *multipart.FileHeader `form:"embedded"`
	}
	var request struct {
		embeddedFiles
		ID     string                  `uri:"id" binding:"required"`
		Name   string                  `form:"name" binding:"required"`
		Token  customParameter         `form:"token"`
		File   *multipart.FileHeader   `form:"file" binding:"required"`
		Value  multipart.FileHeader    `form:"value"`
		Files  []*multipart.FileHeader `form:"files"`
		Array  [2]multipart.FileHeader `form:"array"`
		Nested *nestedFiles
	}
	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)
	for _, field := range []struct{ key, value string }{{key: "name", value: "body"}, {key: "token", value: "custom:ok"}} {
		if err := multipartWriter.WriteField(field.key, field.value); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"file", "value", "files", "files", "array", "array", "nested", "embedded"} {
		part, err := multipartWriter.CreateFormFile(key, key+".txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(part, "content"); err != nil {
			t.Fatal(err)
		}
	}
	if err := multipartWriter.Close(); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.POST("/:id", func(c *core.Context) {
		defer func() {
			if c.Request.MultipartForm != nil {
				_ = c.Request.MultipartForm.RemoveAll()
			}
		}()
		if err := Bind(c, &request).All().Error(); err != nil {
			t.Fatal(err)
		}
		if request.ID != "object" || request.Name != "query" || request.Token.Value != "ok" ||
			request.File == nil ||
			request.Value.Filename != "value.txt" ||
			len(request.Files) != 2 ||
			request.Array[1].Filename != "array.txt" ||
			request.Nested == nil ||
			request.Nested.File == nil ||
			request.Embedded == nil {
			t.Fatalf("multipart mapping lost fields: %#v", request)
		}
		file, err := request.File.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil || string(data) != "content" {
			t.Fatalf("file content = %q, %v", data, err)
		}
		c.Status(200)
	})
	httpRequest := httptest.NewRequest(http.MethodPost, "/object?name=query", &body)
	httpRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	engine.ServeHTTP(httptest.NewRecorder(), httpRequest)
}

func TestAllMultipartDoesNotHideMappingErrors(t *testing.T) {
	for _, test := range []struct {
		name, token string
		fileAtToken bool
	}{
		{name: "invalid custom parameter", token: "invalid"},
		{name: "file at custom parameter", token: "custom:ok", fileAtToken: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			if err := form.WriteField("token", test.token); err != nil {
				t.Fatal(err)
			}
			if test.fileAtToken {
				part, err := form.CreateFormFile("token", "token.txt")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(part, "content"); err != nil {
					t.Fatal(err)
				}
			}
			if err := form.Close(); err != nil {
				t.Fatal(err)
			}
			engine := gin.New()
			engine.POST("/:id", func(c *core.Context) {
				var request struct {
					ID    string          `uri:"id" binding:"required"`
					Token customParameter `form:"token"`
				}
				if err := Bind(c, &request).All().Error(); err == nil {
					t.Fatal("multipart mapping error ignored")
				}
			})
			request := httptest.NewRequest(http.MethodPost, "/object", &body)
			request.Header.Set("Content-Type", form.FormDataContentType())
			engine.ServeHTTP(httptest.NewRecorder(), request)
		})
	}
}

func TestSingleSourceBindingStillValidatesWholeRequest(t *testing.T) {
	for _, source := range []string{"JSON", "Query", "URI"} {
		t.Run(source, func(t *testing.T) {
			engine := gin.New()
			engine.POST("/:id", func(c *core.Context) {
				var request struct {
					Name  string `json:"name" binding:"required"`
					ID    string `uri:"id" binding:"required"`
					Limit int    `form:"limit" binding:"required,min=1"`
				}
				bound := Bind(c, &request)
				switch source {
				case "JSON":
					bound.JSON()
				case "Query":
					bound.Query()
				case "URI":
					bound.Uri()
				}
				if bound.Error() == nil {
					t.Fatal("partial request unexpectedly passed validation")
				}
			})
			request := httptest.NewRequest(
				http.MethodPost,
				"/object?limit=5",
				strings.NewReader(`{"name":"body"}`),
			)
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(httptest.NewRecorder(), request)
		})
	}
}
