package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go"
)

func Feature(obj any) gin.HandlerFunc {
	return func(c *gin.Context) {
		server.OK(c, obj)
	}
}
