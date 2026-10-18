package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// InternalPropagatedIdentity accepts identity forwarded by an upstream internal
// service through x-impersonate-target-* headers. Install it after the caller is
// authenticated as a service account (for example BearerTokens). authorize must
// confirm the caller may forward identity; it must not write identity keys.
//
// When propagation headers are present, tenant, user, and project from the headers
// are written to KeyTenantID, KeyUserID, and KeyProjectID, and the same values
// are stored in KeyImpersonateOrigin* so handlers can tell the identity was
// verified upstream and must not be re-resolved through tenant.
//
// When the headers are absent the middleware is a no-op.
func InternalPropagatedIdentity(authorize func(c *core.Context) error) (core.HandlerFunc, error) {
	if authorize == nil {
		return nil, errors.New("internal propagated identity requires a service authorizer")
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
				"complete propagated identity required",
			)
			return
		}
		if err := authorize(c); err != nil {
			serrors.Abort(c, http.StatusForbidden, "AccessDenied", "internal propagation is not permitted")
			return
		}
		c.Set(constants.KeyTenantID, tenantID)
		c.Set(constants.KeyUserID, userID)
		c.Set(constants.KeyProjectID, projectID)
		c.Set(constants.KeyImpersonateOriginTenantID, tenantID)
		c.Set(constants.KeyImpersonateOriginUserID, userID)
		c.Set(constants.KeyImpersonateOriginProjectID, projectID)
		c.Set(constants.KeyAuthMode, "Internal")
		c.Next()
	}, nil
}
