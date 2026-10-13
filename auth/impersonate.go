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
func Impersonate(authorize func(c *core.Context, orgID, userID string) error) (core.HandlerFunc, error) {
	if authorize == nil {
		return nil, errors.New("impersonation requires a service account authorizer")
	}
	return func(c *core.Context) {
		orgID := strings.TrimSpace(c.GetHeader(constants.HeaderImpersonateOrgID))
		userID := strings.TrimSpace(c.GetHeader(constants.HeaderImpersonateUserID))
		if orgID == "" && userID == "" {
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
		if orgID == "" || userID == "" {
			serrors.Abort(
				c,
				http.StatusForbidden,
				"AccessDenied",
				"complete impersonation identity required",
			)
			return
		}
		if err := authorize(c, orgID, userID); err != nil {
			serrors.Abort(c, http.StatusForbidden, "AccessDenied", "impersonation is not permitted")
			return
		}
		c.Set(constants.KeyImpersonateOriginOrgID, c.GetString(constants.KeyOrgID))
		c.Set(constants.KeyImpersonateOriginUserID, c.GetString(constants.KeyUserID))
		c.Set(constants.KeyOrgID, orgID)
		c.Set(constants.KeyUserID, userID)
		c.Set(constants.KeyAuthMode, "Impersonate")
		c.Next()
	}, nil
}
