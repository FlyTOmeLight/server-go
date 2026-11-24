package swagger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestSwaggerHandlersCapturePrefix(t *testing.T) {
	original := DocPrefix
	t.Cleanup(func() { DocPrefix = original })
	const fallbackPrefix = "swagger-fallback"
	DocPrefix = fallbackPrefix
	fallback := SwaggerWithPrefix("")
	DocPrefix = "swagger-legacy"
	for _, prefix := range []string{"swagger-first", "swagger-second", fallbackPrefix, DocPrefix} {
		if swag.GetSwagger(prefix) == nil {
			swag.Register(prefix, &swag.Spec{SwaggerTemplate: `{"title":"` + prefix + `"}`})
		}
	}
	handlers := []struct {
		name, prefix string
		handler      core.HandlerFunc
	}{
		{name: "first", prefix: "swagger-first", handler: SwaggerWithPrefix("swagger-first")},
		{name: "second", prefix: "swagger-second", handler: SwaggerWithPrefix("swagger-second")},
		{name: "empty", prefix: fallbackPrefix, handler: fallback},
		{name: "legacy", prefix: DocPrefix, handler: Swagger()},
	}
	DocPrefix = "other-instance"
	for _, test := range handlers {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			engine := gin.New()
			path := "/" + test.name + "/swagger/"
			engine.GET(path+"*any", test.handler)
			doc := httptest.NewRecorder()
			engine.ServeHTTP(doc, httptest.NewRequest(http.MethodGet, path+"doc.json", nil))
			if want := `{"title":"` + test.prefix + `"}`; doc.Code != http.StatusOK ||
				doc.Body.String() != want {
				t.Fatalf("doc.json = HTTP %d %s, want %s", doc.Code, doc.Body, want)
			}
			asset := httptest.NewRecorder()
			engine.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, path+"swagger-ui.css", nil))
			if asset.Code != http.StatusOK {
				t.Fatalf("swagger-ui.css = HTTP %d", asset.Code)
			}
		})
	}
}
