package middleware

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func RequestID() core.HandlerFunc {
	return func(c *core.Context) {
		_ = GetRequestID(c)
		c.Next()
	}
}

func GetRequestID(c *core.Context) string {
	requestID := strings.TrimSpace(c.GetHeader(constants.HeaderRequestID))
	if !validRequestID(requestID) {
		requestID = uuid.NewString()
	}
	c.Request.Header.Set(constants.HeaderRequestID, requestID)
	c.Header(constants.HeaderRequestID, requestID)

	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), constants.KeyRequestID, requestID))
	c.Set(constants.KeyRequestID, requestID)
	c.Set(constants.HeaderRequestID, requestID)
	return requestID
}

func validRequestID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < 0x21 || id[i] > 0x7e {
			return false
		}
	}
	return true
}
