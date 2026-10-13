package middleware

import (
	"net/http"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func Ping() core.HandlerFunc {
	return func(c *core.Context) {
		c.String(http.StatusOK, "pong")
	}
}
