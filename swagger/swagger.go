package swagger

import (
	"context"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

var DocPrefix = "unknown"

func Swagger() core.HandlerFunc {
	return ginSwagger.WrapHandler(swaggerFiles.Handler, func(config *ginSwagger.Config) {
		logger.Debugf(context.Background(), "SwaggerInstanceName: %s", DocPrefix)
		config.InstanceName = DocPrefix
	})
}
