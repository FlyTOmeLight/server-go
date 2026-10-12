package swagger

import (
	"context"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/FlyTOmeLight/server-go/logger"
)

var DocPrefix = "unknown"

func Swagger() gin.HandlerFunc {
	return ginSwagger.WrapHandler(swaggerFiles.Handler, func(config *ginSwagger.Config) {
		logger.Debugf(context.Background(), "SwaggerInstanceName: %s", DocPrefix)
		config.InstanceName = DocPrefix
	})
}
