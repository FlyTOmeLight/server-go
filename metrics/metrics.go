package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func Metrics() core.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}
