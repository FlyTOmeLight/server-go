package main

import (
	"context"

	server "github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/gin-gonic/gin"
)

type helloService struct{ engine *core.Engine }

func (s *helloService) Initialize(engine *core.Engine) { s.engine = engine }
func (s *helloService) StartAsync() {
	s.engine.GET("/hello", func(c *gin.Context) { c.JSON(200, gin.H{"message": "hello"}) })
}
func (s *helloService) StopAsync() {}

func main() {
	defer logger.Sync()
	bootstrap := server.NewBootstrap()
	if err := bootstrap.Initialize(); err != nil {
		logger.Fatalf(context.Background(), "initialize: %v", err)
	}
	defer bootstrap.Close()
	h := host.NewHTTPHost(conf.Default())
	h.AddService(&helloService{})
	h.Run()
}
