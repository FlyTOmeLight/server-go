package test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/FlyTOmeLight/server-go/cli"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/FlyTOmeLight/server-go/examples/example-service1/command"
	"github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
	"github.com/FlyTOmeLight/server-go/examples/example-service1/service"
)

const (
	sdkProjectName = "sdkProjectName"
	sdkDatasetName = "sdkDatasetName"
)

var (
	client *trainv1.Client
	cmd    *cobra.Command
)

func TestMain(m *testing.M) {
	_ = os.Remove("ut.db")
	_ = os.RemoveAll("log")

	initServer()
	initCommand()

	code := m.Run()
	os.Exit(code)
}

func initServer() {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	err = db.SetDefaultDBWithConfig(&db.Config{
		Driver: db.DriverSQLite,
		DB:     "ut.db",
	})
	if err != nil {
		panic(err)
	}

	appConfig := &conf.Config{
		AppName: "example-service1",
		RunMode: conf.RunModeDebug,
		HTTPServer: conf.HTTPServerConfig{
			Listen: l.Addr().String(),
		},
	}
	h := host.NewHTTPHost(appConfig)
	h.AddService(service.NewExampleService())
	fmt.Println(appConfig.HTTPServer.Listen)

	go func() {
		_ = l.Close()
		h.Run()
	}()

	h.LoopPing(500*time.Millisecond, 10)

	client, err = trainv1.NewClient("test-ak", "test-sk", "http://"+appConfig.HTTPServer.Listen)
	if err != nil {
		panic(err)
	}
}

func initCommand() {
	c := cli.New()
	c.Append(&command.DatasetCommandBuilder{})

	viper.Set("core.access_key", "test-ak")
	viper.Set("core.secret_key", "test-sk")
	viper.Set("core.endpoint", client.Config.Endpoint)

	cmd = c.Root
}
