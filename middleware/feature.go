package middleware

import (
	"github.com/FlyTOmeLight/server-go"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func Feature(obj any) core.HandlerFunc {
	return func(c *core.Context) {
		server.OK(c, obj)
	}
}
