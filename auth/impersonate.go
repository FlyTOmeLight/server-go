package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// Impersonate must follow authentication. authorize must verify that the current
// caller is a service account allowed to act as the requested tenant and user.
// Without impersonation headers the authenticated identity is left unchanged.
// The project header is optional and replaces KeyProjectID when present.
func Impersonate(authorize func(c *core.Context, tenantID, userID string) error) (core.HandlerFunc, error) {
	if authorize == nil {
		return nil, errors.New("impersonation requires a service account authorizer")
	}
	return func(c *core.Context) {
		tenantID := strings.TrimSpace(c.GetHeader(constants.HeaderImpersonateTenantID))
		userID := strings.TrimSpace(c.GetHeader(constants.HeaderImpersonateUserID))
		projectID := strings.TrimSpace(c.GetHeader(constants.HeaderImpersonateProjectID))
		if tenantID == "" && userID == "" {
			c.Next()
			return
		}

		if c.GetString(constants.KeyUserID) == "" {
			serrors.Abort(
				c,
				http.StatusUnauthorized,
				"Unauthorized",
				"authenticated service account required",
			)
			return
		}
		if tenantID == "" || userID == "" {
			serrors.Abort(
				c,
				http.StatusForbidden,
				"AccessDenied",
				"complete impersonation identity required",
			)
			return
		}
		if err := authorize(c, tenantID, userID); err != nil {
			serrors.Abort(c, http.StatusForbidden, "AccessDenied", "impersonation is not permitted")
			return
		}
		c.Set(constants.KeyImpersonateOriginTenantID, c.GetString(constants.KeyTenantID))
		c.Set(constants.KeyImpersonateOriginUserID, c.GetString(constants.KeyUserID))
		c.Set(constants.KeyImpersonateOriginProjectID, c.GetString(constants.KeyProjectID))
		c.Set(constants.KeyTenantID, tenantID)
		c.Set(constants.KeyUserID, userID)
		if projectID != "" {
			c.Set(constants.KeyProjectID, projectID)
		}
		c.Set(constants.KeyAuthMode, "Impersonate")
		c.Next()
	}, nil
}
