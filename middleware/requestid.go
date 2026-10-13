package middleware

import (
	"context"

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
	requestID := c.GetHeader(constants.HeaderRequestID)
	if requestID == "" {
		requestID = uuid.NewString()
		c.Request.Header.Add(constants.HeaderRequestID, requestID)
	}

	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), constants.KeyRequestID, requestID))
	c.Set(constants.KeyRequestID, requestID)
	c.Set(constants.HeaderRequestID, requestID)
	return requestID
}
