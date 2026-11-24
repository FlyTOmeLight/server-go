package db_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/FlyTOmeLight/server-go/db"
	serrors "github.com/FlyTOmeLight/server-go/errors"
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
			want: serrors.ErrorResourceNotFound,
		},
		{
			desc: "resource is exists(mysql)",
			err: &mysql.MySQLError{
				Number: 1062,
			},
			want: serrors.ErrorResourceIsExists,
		},
		{
			desc: "resource is exists(sqlite)",
			err:  errors.New("UNIQUE constraint failed"),
			want: serrors.ErrorResourceIsExists,
		},
		{
			desc: "unique violation (postgresql)",
			err:  fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505"}),
			want: serrors.ErrorResourceIsExists,
		},
		{
			desc: "invalid text representation (postgresql)",
			err:  fmt.Errorf("select: %w", &pgconn.PgError{Code: "22P02"}),
			want: serrors.NewInvalidParameter("invalid input syntax"),
		},
		{
			desc: "invalid order by",
			err: &mysql.MySQLError{
				Number:  1054,
				Message: "Unknown column 'key' in 'order clause'",
			},
			want: serrors.NewInvalidParameter("Unknown field used in orderBy"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := db.NewError(tc.err)
			assert.Equal(t, tc.want, actual)
		})
	}
}

func TestNewDBErrorPreservesUnknownCause(t *testing.T) {
	cause := &pgconn.PgError{Code: "XX000", Message: "database details"}
	original := fmt.Errorf("query: %w", cause)
	result := db.NewError(original)
	assert.True(t, serrors.IsStatusCode(result, http.StatusInternalServerError))
	assert.True(t, serrors.IsCode(result, "InternalError"))
	assert.ErrorIs(t, result, original)
	var pgError *pgconn.PgError
	if assert.ErrorAs(t, result, &pgError) {
		assert.Same(t, cause, pgError)
	}
	normalized := serrors.New(result)
	assert.Equal(t, http.StatusInternalServerError, normalized.Status)
	assert.Equal(t, "InternalError", normalized.Code)
	assert.Equal(t, serrors.ErrorInternalError.Error(), normalized.Message)
	assert.ErrorIs(t, normalized, original)
}
