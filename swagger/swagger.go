package swagger

import (
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

var DocPrefix = "unknown"

func Swagger() core.HandlerFunc {
	return SwaggerWithPrefix("")
}

// SwaggerWithPrefix captures the document instance for a Swagger handler.
// An empty prefix uses DocPrefix at registration time.
func SwaggerWithPrefix(prefix string) core.HandlerFunc {
	if prefix == "" {
		prefix = DocPrefix
	}
	files := *swaggerFiles.Handler
	return ginSwagger.WrapHandler(&files, ginSwagger.InstanceName(prefix))
}
