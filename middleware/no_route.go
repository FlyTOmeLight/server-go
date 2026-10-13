package middleware

import (
	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func APINotFound() core.HandlerFunc {
	return func(c *core.Context) {
		c.Abort()
		_ = c.Error(errors.ErrorAPINotFound)
	}
}
