package mongodb

import (
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"

	serrors "github.com/FlyTOmeLight/server-go/errors"
)

// NewError translates MongoDB errors into framework errors.
func NewError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return serrors.ErrorResourceNotFound
	case mongo.IsDuplicateKeyError(err):
		return serrors.ErrorResourceIsExists
	default:
		return fmt.Errorf("%w: %w", serrors.ErrorInternalError, err)
	}
}
