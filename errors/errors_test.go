package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FlyTOmeLight/server-go/conf"
)

func TestWrap(t *testing.T) {
	testCases := []struct {
		desc           string
		err            error
		wrapMessage    string
		shouldContains bool
	}{
		{
			desc:           "serverGo error",
			err:            ErrorResourceNotFound,
			wrapMessage:    "filesystem not found",
			shouldContains: true,
		},
		{
			desc:           "not serverGo error",
			err:            errors.New("a wild error"),
			wrapMessage:    "filesystem not found",
			shouldContains: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := Wrap(tc.err, tc.wrapMessage)
			t.Log(err.Error())
			if strings.Contains(err.Error(), tc.wrapMessage) != tc.shouldContains {
				t.Fatalf("case: %#v", tc)
			}
		})
	}
}

func TestWrappedFrameworkErrors(t *testing.T) {
	original := &Error{
		Status:     http.StatusNotFound,
		StatusCode: 100404,
		Code:       "Missing",
		Message:    "gone",
		RequestID:  "upstream",
	}
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "standard wrapping", err: fmt.Errorf("load object: %w", original)},
		{name: "joined", err: errors.Join(errors.New("additional failure"), original)},
		{name: "framework wrap", err: Wrap(fmt.Errorf("load object: %w", original), "request failed")},
		{name: "framework message", err: WithMessage(fmt.Errorf("load object: %w", original), "object unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !IsStatusCode(test.err, 404) || !IsCode(test.err, "Missing") ||
				!errors.Is(test.err, original) {
				t.Fatal("framework error identity was lost")
			}
			detail := New(test.err)
			if detail == original || detail.Status != 404 || detail.StatusCode != 100404 ||
				detail.Code != "Missing" ||
				detail.RequestID != "upstream" || !errors.Is(detail, original) {
				t.Fatalf("normalized error: %#v", detail)
			}
			engine := gin.New()
			engine.Use(ErrorHandler(&conf.Config{}))
			engine.GET("/", func(c *gin.Context) { _ = c.Error(test.err) })
			writer := httptest.NewRecorder()
			engine.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/", nil))
			var response Error
			if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if writer.Code != 404 || response.Code != "Missing" || response.StatusCode != 100404 {
				t.Fatalf("HTTP error: %d %s", writer.Code, writer.Body)
			}
			if original.RequestID != "upstream" || original.Message != "gone" {
				t.Fatal("HTTP normalization mutated original error")
			}
		})
	}
	if Wrap(nil, "context") != nil || WithMessage(nil, "context") != nil {
		t.Fatal("wrapping nil must preserve success")
	}
}

func TestWrappedInternalErrorsDoNotExposeDetails(t *testing.T) {
	err := fmt.Errorf("private operation: %w", NewError(500, "InternalError", "private database detail"))
	if normalized := New(err); normalized.Message != ErrorInternalError.Error() {
		t.Fatalf("internal details exposed: %s", normalized.Message)
	}
	if normalized := normalize(err, true); normalized.Message != err.Error() {
		t.Fatal("debug normalization lost error context")
	}
}

func TestIsStatusCode(t *testing.T) {
	type args struct {
		err        error
		targetCode int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "server-go error",
			args: args{
				err:        ErrorResourceNotFound,
				targetCode: http.StatusNotFound,
			},
			want: true,
		},
		{
			name: "std error",
			args: args{
				err:        errors.New("std error"),
				targetCode: http.StatusNotFound,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(
				t,
				tt.want,
				IsStatusCode(tt.args.err, tt.args.targetCode),
				"IsStatusCode(%v, %v)",
				tt.args.err,
				tt.args.targetCode,
			)
		})
	}
}

func TestError(t *testing.T) {
	err := NewError(400, "ErrorCode", "this is an error message")
	require.NotNil(t, err)
	newErr := New(err)
	require.NotNil(t, newErr)
	require.Equal(t, 400, newErr.Status)
	newErr.SetStatusCode(0)
	require.Equal(t, 400, newErr.StatusCode)

	err = NewError(400, "ErrorCode", "this is an error message")
	require.NotNil(t, err)
	newErr = New(err)
	require.NotNil(t, newErr)
	require.Equal(t, 400, newErr.Status)
	newErr.SetStatusCode(345)
	require.Equal(t, 345400, newErr.StatusCode)
}
