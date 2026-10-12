package middleware

import (
	"net/http"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/gin-gonic/gin"
)

func Healthz() gin.HandlerFunc {
	return func(c *gin.Context) {
		if conf.Default().LoadMode == conf.LoadModeLoaded {
			c.String(http.StatusOK, "healthz")
			return
		}

		c.Abort()
		_ = c.Error(errors.NewError(http.StatusServiceUnavailable, "ServiceLoading", "The service is loading, please try again later."))
	}
}
