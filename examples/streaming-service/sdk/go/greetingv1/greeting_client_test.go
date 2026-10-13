package greetingv1

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	frameworkerrors "github.com/FlyTOmeLight/server-go/errors"
)

func TestOpenPreservesHTTPFailures(t *testing.T) {
	for _, test := range []struct {
		name, body, code  string
		status, appStatus int
	}{
		{name: "framework error", status: 403, appStatus: 100403, body: `{"code":"Denied","statusCode":100403,"requestId":"request-1"}`, code: "Denied"},
		{name: "HTML error", status: 502, appStatus: 502, body: "<html>upstream failed</html>", code: "HTTPError"},
		{name: "invalid JSON error", status: 503, appStatus: 503, body: "{", code: "HTTPError"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			body, err := client.Open(t.Context(), "reader")
			if body != nil {
				body.Close()
				t.Fatal("HTTP failure returned a live stream")
			}
			var detail *frameworkerrors.Error
			if !errors.As(err, &detail) || detail.Status != test.status ||
				detail.StatusCode != test.appStatus ||
				detail.Code != test.code {
				t.Fatalf("error = %+v", err)
			}
			if test.code == "Denied" && detail.RequestID != "request-1" {
				t.Fatal("request ID lost")
			}
		})
	}
}
