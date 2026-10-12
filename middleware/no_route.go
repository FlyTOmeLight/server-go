package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/errors"
)

func APINotFound() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Abort()
		_ = c.Error(errors.ErrorAPINotFound)
	}
}
