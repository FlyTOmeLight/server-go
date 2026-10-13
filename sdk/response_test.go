package sdk

import (
	"strings"
	"testing"
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
