package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// TrustedIdentityHeaders accepts identity headers only after verify authenticates
// the gateway and checks that it controls those headers. verify must not trust the
// identity headers themselves. Existing authenticated identities are not replaced.
// The project header is optional; when present it is published as KeyProjectID.
func TrustedIdentityHeaders(verify func(*http.Request) error) (core.HandlerFunc, error) {
	if verify == nil {
		return nil, errors.New("trusted identity headers require a gateway verifier")
	}
	return func(c *core.Context) {
		if err := verify(c.Request); err != nil {
			serrors.Abort(c, http.StatusUnauthorized, "Unauthorized", "untrusted identity source")
			return
		}
		if c.GetString(constants.KeyUserID) != "" || c.GetString(constants.KeyTenantID) != "" {
			serrors.Abort(
				c,
				http.StatusForbidden,
				"AccessDenied",
				"authenticated identity cannot be replaced",
			)
			return
		}
		tenantID := strings.TrimSpace(c.GetHeader(constants.HeaderTenantID))
		userID := strings.TrimSpace(c.GetHeader(constants.HeaderUserID))
		projectID := strings.TrimSpace(c.GetHeader(constants.HeaderProjectID))
		if tenantID == "" || userID == "" {
			serrors.Abort(c, http.StatusUnauthorized, "Unauthorized", "complete identity headers required")
			return
		}
		c.Set(constants.KeyTenantID, tenantID)
		c.Set(constants.KeyUserID, userID)
		c.Set(constants.KeyProjectID, projectID)
		c.Next()
	}, nil
}
