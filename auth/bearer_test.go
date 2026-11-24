package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestBearerTokensOwnsCredentials(t *testing.T) {
	tokens := map[string]string{
		"credential": "user", "longer-service-credential": "service", "": "invalid", "empty-user": "",
	}
	middleware := BearerTokens(tokens)
	tokens["credential"] = "changed"
	tokens["later"] = "other"
	delete(tokens, "longer-service-credential")
	engine := gin.New()
	engine.GET("/", middleware, func(c *core.Context) { c.String(200, UserID(c)) })
	for _, tc := range []struct {
		header, body string
		status       int
	}{
		{header: "Bearer credential", body: "user", status: 200},
		{header: "Bearer longer-service-credential", body: "service", status: 200},
		{header: "Bearer later", status: 401},
		{header: "Bearer wrong", status: 401},
		{header: "Bearer empty-user", status: 401},
		{header: "Bearer ", status: 401},
		{header: "", status: 401},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", tc.header)
		engine.ServeHTTP(w, req)
		if w.Code != tc.status || (tc.status == 200 && w.Body.String() != tc.body) {
			t.Errorf("header %q: status=%d body=%s", tc.header, w.Code, w.Body.String())
		}
	}
}

func TestParseBearerTokens(t *testing.T) {
	for _, raw := range []string{"", `{}`, `{"short":"user"}`, `{"123456789012345678901234":" "}`} {
		if _, err := ParseBearerTokens(raw); err == nil {
			t.Fatalf("invalid credentials accepted")
		}
	}
	tokens, err := ParseBearerTokens(`{"123456789012345678901234":"user"}`)
	if err != nil || tokens["123456789012345678901234"] != "user" {
		t.Fatalf("tokens=%v err=%v", tokens, err)
	}
}
