package console

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FlyTOmeLight/server-go/conf"
)

func TestConsoleProxyCapturesTargetAndAppCode(t *testing.T) {
	config := &conf.HTTPServerConfig{Listen: "first.example:8080"}
	proxy := NewConsoleProxy(config, 731)
	config.Listen = "second.example:8081"
	req := httptest.NewRequest(http.MethodPost, "http://frontend/example/api/v1/Create", strings.NewReader(`{}`))
	proxy.Director(req)
	if req.URL.Host != "first.example:8080" || req.URL.Path != "/example/internal/v1/Create" ||
		req.Method != http.MethodPost {
		t.Fatalf("incorrect proxy target: %s %s", req.Method, req.URL)
	}
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{},
		Request:    req,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}
	if err := proxy.ModifyResponse(resp); err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"statusCode":731500`) {
		t.Fatalf("missing explicit app code: status=%d body=%s", resp.StatusCode, body)
	}
}
