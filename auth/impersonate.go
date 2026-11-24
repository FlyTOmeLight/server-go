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
//
// For internal service chains that forward an already verified end-user identity,
// use InternalPropagatedIdentity instead.
func Impersonate(authorize func(c *core.Context, tenantID, userID string) error) (core.HandlerFunc, error) {
	if authorize == nil {
		return nil, errors.New("impersonation requires a service account authorizer")
	}
	return func(c *core.Context) {
		if hasMultipleImpersonationHeaderValues(c) {
			serrors.Abort(
				c,
				http.StatusForbidden,
				"AccessDenied",
				"multiple values for impersonation headers",
			)
			return
		}
		tenantID := strings.TrimSpace(c.GetHeader(constants.HeaderImpersonateTenantID))
		userID := strings.TrimSpace(c.GetHeader(constants.HeaderImpersonateUserID))
		projectID := strings.TrimSpace(c.GetHeader(constants.HeaderImpersonateProjectID))
		if tenantID == "" && userID == "" {
			if len(c.Request.Header.Values(constants.HeaderImpersonateProjectID)) == 0 {
				c.Next()
				return
			}
		} else if c.GetString(constants.KeyUserID) == "" {
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
		c.Set(constants.KeyUserName, "")
		c.Set(constants.KeyUserRole, "")
		if projectID != "" {
			c.Set(constants.KeyProjectID, projectID)
		}
		c.Set(constants.KeyAuthMode, "Impersonate")
		SyncIdentityToRequestContext(c)
		c.Next()
	}, nil
}

func hasMultipleImpersonationHeaderValues(c *core.Context) bool {
	return hasMultipleHeaderValues(
		c,
		constants.HeaderImpersonateTenantID,
		constants.HeaderImpersonateUserID,
		constants.HeaderImpersonateProjectID,
	)
}
