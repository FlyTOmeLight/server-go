package host

import (
	"context"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// Service installs routes separately from owned background resources.
// RegisterRoutes may run for a fresh router; it must not create workers or connections.
type Service interface {
	RegisterRoutes(*core.RouterGroup) error
	Start(context.Context) error
	Stop(context.Context) error
}
