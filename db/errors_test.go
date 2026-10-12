package db_test

import (
	"errors"
	"github.com/FlyTOmeLight/server-go/db"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"testing"
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
