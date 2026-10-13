package server

import (
	"errors"
	"net/http"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func Abort(c *core.Context, err error) {
	if errDetail, ok := errors.AsType[*serrors.Error](err); ok {
		c.Status(errDetail.Status)
		c.Abort()
		_ = c.Error(errDetail)
		return
	}

	c.Status(http.StatusInternalServerError)
	c.Abort()
	_ = c.Error(err)
}

func OK(c *core.Context, resp any) {
	if resp == nil {
		c.Data(http.StatusOK, constants.ContentTypeJSON, nil)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// OKN means message delivered in Dapr
func OKN(c *core.Context) {
	c.Data(http.StatusNoContent, constants.ContentTypeJSON, nil)
}

// Forbidden means message forbidden by access controls in Dapr
func Forbidden(c *core.Context) {
	c.Data(http.StatusForbidden, constants.ContentTypeJSON, nil)
}

// NotFound means no pubsub name or topic give in Dapr
func NotFound(c *core.Context) {
	c.Data(http.StatusNotFound, constants.ContentTypeJSON, nil)
}

// InternalError means Delivery failed in Dapr
func InternalError(c *core.Context) {
	c.Data(http.StatusInternalServerError, constants.ContentTypeJSON, nil)
}
