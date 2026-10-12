package errors

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/FlyTOmeLight/server-go/conf"
)

func TestNewDBError(t *testing.T) {
	testcases := []struct {
		desc string
		err  error
		want error
	}{
		{
			desc: "nil",
			want: nil,
		},
		{
			desc: "record not found",
			err:  gorm.ErrRecordNotFound,
			want: ErrorResourceNotFound,
		},
		{
			desc: "resource is exists(mysql)",
			err: &mysql.MySQLError{
				Number: 1062,
			},
			want: ErrorResourceIsExists,
		},
		{
			desc: "resource is exists(sqlite)",
			err:  errors.New("UNIQUE constraint failed"),
			want: ErrorResourceIsExists,
		},
		{
			desc: "invalid order by",
			err: &mysql.MySQLError{
				Number:  1054,
				Message: "Unknown column 'key' in 'order clause'",
			},
			want: NewInvalidParameter("Unknown field used in orderBy"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := NewDBError(tc.err)
			assert.Equal(t, tc.want, actual)
		})
	}
}

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
			assert.Equalf(t, tt.want, IsStatusCode(tt.args.err, tt.args.targetCode), "IsStatusCode(%v, %v)", tt.args.err, tt.args.targetCode)
		})
	}
}

func TestError(t *testing.T) {
	err := NewError(400, "ErrorCode", "this is an error message")
	require.NotNil(t, err)
	newErr := New(err)
	require.NotNil(t, newErr)
	require.Equal(t, 400, newErr.Status)
	newErr.SetStatusCode()
	require.Equal(t, 400, newErr.StatusCode)

	err = conf.SetDefault(&conf.Config{AppName: "Test", AppCode: 345, RunMode: "debug"})
	require.NoError(t, err)

	err = NewError(400, "ErrorCode", "this is an error message")
	require.NotNil(t, err)
	newErr = New(err)
	require.NotNil(t, newErr)
	require.Equal(t, 400, newErr.Status)
	newErr.SetStatusCode()
	require.Equal(t, 345400, newErr.StatusCode)
}
