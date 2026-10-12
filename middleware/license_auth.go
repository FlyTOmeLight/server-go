package middleware

import (
	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/gin-gonic/gin"
	"net/http"
)

// LicenseAuthorizer reports the current license state. The caller owns its lifecycle.
type LicenseAuthorizer interface {
	GetAuthorization() bool
	ErrorMessage() string
}

// LicenseAuth checks a license client that has already been initialized by the caller.
func LicenseAuth(client LicenseAuthorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if client == nil {
			errors.AbortWithError(c, errors.NewError(http.StatusInternalServerError, "LicenseAuthFailed", "license鉴权客户端未初始化"))
			return
		}
		if !client.GetAuthorization() {
			core.HTTPLogger.Errorf(c, "LicenseAuthError err: %s", client.ErrorMessage())
			errors.AbortWithError(c, errors.NewError(http.StatusUnauthorized, "LicenseAuthFailed", "license鉴权失败，未获得授权"))
			return
		}
		c.Next()
	}
}
