package auth

import (
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
	credentials := make(map[string]string, len(tokens))
	for token, user := range tokens {
		if token != "" && user != "" {
			credentials[token] = user
		}
	}
	return func(c *core.Context) {
		value := c.GetHeader("Authorization")
		if !strings.HasPrefix(value, "Bearer ") {
			errors.Abort(c, http.StatusUnauthorized, "Unauthorized", "bearer token required")
			return
		}
		token := strings.TrimPrefix(value, "Bearer ")
		for candidate, user := range credentials {
			if subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1 {
				c.Set(constants.KeyUserID, user)
				c.Next()
				return
			}
		}
		errors.Abort(c, http.StatusUnauthorized, "Unauthorized", "invalid bearer token")
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
