package host

import (
	"context"
	"github.com/gin-gonic/gin"
)

// Service installs routes separately from owned background resources.
// RegisterRoutes may run for a fresh router; it must not create workers or connections.
type Service interface {
	RegisterRoutes(*gin.RouterGroup) error
	Start(context.Context) error
	Stop(context.Context) error
}
