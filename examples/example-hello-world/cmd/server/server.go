package main

import (
	"context"

	"github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/logger"

	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/service"
)

func main() {
	defer logger.Sync()

	// init bootstrap
	// NewBootstrap传入需要启动的Modules
	// Module中的Name参数用于指定配置文件中的key, 如果不传则为Module中的默认值, 例如db.Module的defaultName = DB
	// 也可以自行构造Module的Config, 这时Module不会去配置文件中解析配置, 而是使用传入的自定义配置
	bootstrap := server.NewBootstrap(&db.Module{})
	err := bootstrap.Initialize()
	if err != nil {
		logger.Fatalf(context.Background(), "init bootstrap error: %v", err)
	}
	defer bootstrap.Close()

	// host & services
	h := host.NewHTTPHost(conf.Default())
	svc := service.NewHelloWorldService(nil)
	h.AddService(svc)
	h.Run()
}
