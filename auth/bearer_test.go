package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBearerTokensOwnsCredentials(t *testing.T) {
	tokens := map[string]string{"credential": "user"}
	middleware := BearerTokens(tokens)
	tokens["credential"] = "changed"
	tokens["later"] = "other"
	engine := gin.New()
	engine.GET("/", middleware, func(c *gin.Context) { c.String(200, UserID(c)) })
	for _, tc := range []struct {
		header, body string
		status       int
	}{
		{header: "Bearer credential", body: "user", status: 200}, {header: "Bearer later", body: "", status: 401}, {header: "Bearer wrong", body: "", status: 401}, {header: "", body: "", status: 401},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", tc.header)
		engine.ServeHTTP(w, req)
		if w.Code != tc.status || (tc.status == 200 && w.Body.String() != tc.body) {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
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
