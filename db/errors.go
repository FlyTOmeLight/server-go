package db

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"

	serrors "github.com/FlyTOmeLight/server-go/errors"
)

var dbOrderErrMsgRegex = regexp.MustCompile("^Unknown column '(?P<column>.+?)' in 'order clause'$")

func NewError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return serrors.ErrorResourceNotFound
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return serrors.ErrorResourceIsExists
	}
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1366 {
		return serrors.ErrorIncorrectString
	}

	if strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
		return serrors.ErrorResourceIsExists
	}

	// unknown column in order clause
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1054 {
		m := dbOrderErrMsgRegex.FindStringSubmatch(mysqlErr.Message)
		if len(m) > dbOrderErrMsgRegex.NumSubexp() {
			return serrors.NewInvalidParameter("Unknown field used in orderBy")
		}
	}

	// mongodb error
	if errors.Is(err, mongo.ErrNoDocuments) {
		return serrors.ErrorResourceNotFound
	}
	if mongo.IsDuplicateKeyError(err) {
		return serrors.ErrorResourceIsExists
	}

	return serrors.NewError(http.StatusInternalServerError, "InternalError", err.Error())
}
