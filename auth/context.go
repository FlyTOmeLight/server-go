package auth

import (
	"context"

	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// Identity is the authenticated identity and its origin, when applicable.
type Identity struct {
	TenantID        string
	UserID          string
	ProjectID       string
	UserName        string
	UserRole        string
	AuthMode        string
	OriginTenantID  string
	OriginUserID    string
	OriginProjectID string
}

type identityContextKey struct{}

// WithIdentity returns a context carrying a copy of id.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, id)
}

// IdentityFromContext returns the identity attached to ctx, if present.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	if ctx == nil {
		return Identity{}, false
	}
	id, ok := ctx.Value(identityContextKey{}).(Identity)
	return id, ok
}

// TenantID returns the authenticated tenant ID from the Gin context.
func TenantID(c *core.Context) string { return c.GetString(constants.KeyTenantID) }

// ProjectID returns the authenticated project ID from the Gin context.
func ProjectID(c *core.Context) string { return c.GetString(constants.KeyProjectID) }

// AuthMode returns the authentication mode from the Gin context.
func AuthMode(c *core.Context) string { return c.GetString(constants.KeyAuthMode) }

// IdentityFromGin reads the identity fields published in the Gin context.
func IdentityFromGin(c *core.Context) Identity {
	return Identity{
		TenantID:        TenantID(c),
		UserID:          UserID(c),
		ProjectID:       ProjectID(c),
		UserName:        c.GetString(constants.KeyUserName),
		UserRole:        c.GetString(constants.KeyUserRole),
		AuthMode:        AuthMode(c),
		OriginTenantID:  c.GetString(constants.KeyImpersonateOriginTenantID),
		OriginUserID:    c.GetString(constants.KeyImpersonateOriginUserID),
		OriginProjectID: c.GetString(constants.KeyImpersonateOriginProjectID),
	}
}

// SyncIdentityToRequestContext publishes the current Gin identity to the HTTP request.
func SyncIdentityToRequestContext(c *core.Context) {
	if c.Request != nil {
		c.Request = c.Request.WithContext(WithIdentity(c.Request.Context(), IdentityFromGin(c)))
	}
}
