package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"strings"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// BearerTokens authenticates static service credentials as framework user IDs.
// The handler owns an immutable copy, so request handling never reads caller mutations.
func BearerTokens(tokens map[string]string) core.HandlerFunc {
	type credential struct {
		digest [sha256.Size]byte
		user   string
	}
	credentials := make([]credential, 0, len(tokens))
	for token, user := range tokens {
		if token != "" && user != "" {
			credentials = append(credentials, credential{digest: sha256.Sum256([]byte(token)), user: user})
		}
	}
	return func(c *core.Context) {
		value := c.GetHeader("Authorization")
		if !strings.HasPrefix(value, "Bearer ") {
			errors.Abort(c, http.StatusUnauthorized, "Unauthorized", "bearer token required")
			return
		}
		digest := sha256.Sum256([]byte(strings.TrimPrefix(value, "Bearer ")))
		matched := -1
		for i := range credentials {
			equal := subtle.ConstantTimeCompare(credentials[i].digest[:], digest[:])
			matched = subtle.ConstantTimeSelect(equal, i, matched)
		}
		if matched < 0 {
			errors.Abort(c, http.StatusUnauthorized, "Unauthorized", "invalid bearer token")
			return
		}
		c.Set(constants.KeyUserID, credentials[matched].user)
		SyncIdentityToRequestContext(c)
		c.Next()
	}
}

func UserID(c *core.Context) string { return c.GetString(constants.KeyUserID) }

// ParseBearerTokens validates a nonempty deployment token-to-user JSON map.
func ParseBearerTokens(raw string) (map[string]string, error) {
	var tokens map[string]string
	if err := json.Unmarshal([]byte(raw), &tokens); err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, stderrors.New("bearer credentials are required")
	}
	for token, user := range tokens {
		if len(token) < 24 || strings.TrimSpace(user) == "" {
			return nil, stderrors.New("bearer tokens must be at least 24 characters and identify a user")
		}
	}
	return tokens, nil
}
