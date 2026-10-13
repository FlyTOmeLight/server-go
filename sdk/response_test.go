package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	serrors "github.com/FlyTOmeLight/server-go/errors"
)

func TestDecodeError(t *testing.T) {
	for _, tc := range []struct {
		body, code string
		appStatus  int
	}{
		{body: `{"code":"Denied","message":"no","requestId":"r","statusCode":100403}`, code: "Denied", appStatus: 100403},
		{body: "upstream failed", code: "HTTPError", appStatus: 403},
		{body: `{}`, code: "HTTPError", appStatus: 403},
	} {
		err := DecodeError(403, strings.NewReader(tc.body))
		if err.Status != 403 || err.StatusCode != tc.appStatus || err.Code != tc.code {
			t.Fatalf("error=%+v", err)
		}
		if tc.code == "Denied" && err.RequestID != "r" {
			t.Fatal("request ID lost")
		}
	}
}

func TestCompleteJSONRequest(t *testing.T) {
	for _, test := range []struct {
		name, body, code string
		status           int
		decodeFailure    bool
	}{
		{name: "success", status: 200, body: `{"value":"received"}`},
		{name: "empty success", status: 204},
		{name: "malformed success", status: 200, body: `{`, decodeFailure: true},
		{name: "framework failure", status: 403, body: `{"code":"Denied","statusCode":100403,"requestId":"r"}`, code: "Denied"},
		{name: "non JSON failure", status: 502, body: "upstream failed", code: "HTTPError"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := NewClientWithConfig(&Config{Endpoint: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			var result struct{ Value string }
			response, transportError := client.R().SetResult(&result).Get("/")
			err = Complete(response, transportError)
			switch {
			case test.decodeFailure:
				if err == nil || err != transportError {
					t.Fatalf("decoding error lost: %v", err)
				}
			case test.code != "":
				var detail *serrors.Error
				if !errors.As(err, &detail) || detail.Status != test.status ||
					detail.Code != test.code {
					t.Fatalf("HTTP error lost: %+v", err)
				}
				if test.code == "Denied" && (detail.StatusCode != 100403 || detail.RequestID != "r") {
					t.Fatalf("metadata lost: %+v", detail)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if test.status == 200 && result.Value != "received" {
					t.Fatalf("result lost: %+v", result)
				}
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := NewClientWithConfig(&Config{Endpoint: "http://localhost:1"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	response, err := client.R().SetContext(ctx).Get("/")
	if err := Complete(response, err); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
