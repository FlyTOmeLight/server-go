package auth

import (
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func Use(engine *core.Engine, config *conf.Config) {
	engine.Use(Impersonate())

	// 优先级：Fake > AuthCenter
	switch config.HTTPServer.AuthType {
	case conf.AuthTypeFake:
		engine.Use(FakeAuthWithID(
			config.FakeAuth.OrgID,
			config.FakeAuth.UserID,
			config.FakeAuth.UserName,
			config.FakeAuth.UserRole))
	case conf.AuthTypeAuthCenter:
		engine.Use(AuthWithAuthCenter())
	}
}

func FakeAuth() core.HandlerFunc {
	return FakeAuthWithID("", "", "", "")
}

func FakeAuthWithID(orgID, userID, userName, userRole string) core.HandlerFunc {
	if orgID == "" {
		orgID = "test-org-id"
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
		c.Set(constants.KeyOrgID, orgID)
		c.Set(constants.KeyUserID, userID)
		c.Set(constants.KeyUserName, userName)
		c.Set(constants.KeyUserRole, userRole)
		c.Next()
	}
}

func AuthWithAuthCenter() core.HandlerFunc {
	return func(c *core.Context) {
		orgID := c.GetHeader(constants.HeaderOrgID)
		c.Set(constants.KeyOrgID, orgID)

		userID := c.GetHeader(constants.HeaderUserID)
		c.Set(constants.KeyUserID, userID)

		c.Next()
	}
}
