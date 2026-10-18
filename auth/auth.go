package auth

import (
	"errors"
	"fmt"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// Use installs development identities. Production authentication must be
// installed explicitly, with its verifier, before protected routes.
func Use(engine *core.Engine, config *conf.Config) error {
	if engine == nil || config == nil {
		return errors.New("authentication requires an engine and config")
	}
	switch config.HTTPServer.AuthType {
	case "", conf.AuthTypeFake:
		engine.Use(FakeAuthWithID(
			config.FakeAuth.TenantID,
			config.FakeAuth.UserID,
			config.FakeAuth.UserName,
			config.FakeAuth.UserRole))
	default:
		return fmt.Errorf(
			"authentication mode %q requires an explicit authentication handler",
			config.HTTPServer.AuthType,
		)
	}
	return nil
}

func FakeAuth() core.HandlerFunc {
	return FakeAuthWithID("", "", "", "")
}

func FakeAuthWithID(tenantID, userID, userName, userRole string) core.HandlerFunc {
	if tenantID == "" {
		tenantID = "test-tenant-id"
	}
	if userID == "" {
		userID = "test-user-id"
	}
	if userName == "" {
		userName = "test-user-name"
	}
	if userRole == "" {
		userRole = "test-user-role"
	}
	return func(c *core.Context) {
		c.Set(constants.KeyTenantID, tenantID)
		c.Set(constants.KeyUserID, userID)
		c.Set(constants.KeyUserName, userName)
		c.Set(constants.KeyUserRole, userRole)
		c.Next()
	}
}
