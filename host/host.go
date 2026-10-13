package host

import (
	"context"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// Service installs routes separately from owned background resources.
// RegisterRoutes may run for a fresh router; it must not create workers or connections.
// Start must respond to context cancellation while starting. Stop cancels and
// joins all owned workers; nil confirms dependencies can be released. It must
// handle partial starts and allow retries after errors or an expired caller deadline.
type Service interface {
	RegisterRoutes(*core.RouterGroup) error
	Start(context.Context) error
	Stop(context.Context) error
}
