package test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FlyTOmeLight/server-go/cli"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/httpintegration"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/sdk"
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
	directory, err := os.MkdirTemp("", "service1-test-")
	if err != nil {
		panic(err)
	}
	code := runTests(m, directory)
	os.RemoveAll(directory)
	os.Exit(code)
}

func runTests(m *testing.M, directory string) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	database, err := db.NewDB(ctx, &db.Config{Driver: db.DriverSQLite, DB: filepath.Join(directory, "test.db")})
	if err != nil {
		panic(err)
	}
	pool, err := database.DB()
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	manager, err := conf.NewManagerFromConfig(
		&conf.Config{AppName: "example-service1", AppCode: 100, RunMode: conf.RunModeTest},
	)
	if err != nil {
		panic(err)
	}
	log, err := logger.NewLogger(&logger.Config{})
	if err != nil {
		panic(err)
	}
	defer log.Close()
	svc, err := service.NewExampleService(ctx, database)
	if err != nil {
		panic(err)
	}
	h, err := host.NewHTTPHost(manager, log, httpintegration.Install, svc)
	if err != nil {
		panic(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				panic(err)
			}
		case <-time.After(5 * time.Second):
			panic("host did not stop")
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		address := h.Address()
		if address != "" {
			response, err := http.Get("http://" + address + "/healthz")
			if err == nil {
				response.Body.Close()
				if response.StatusCode == 200 {
					break
				}
			}
		}
		if time.Now().After(deadline) {
			panic("host did not become ready")
		}
		time.Sleep(time.Millisecond)
	}
	client, err = trainv1.NewClientWithConfig(&sdk.Config{Endpoint: "http://" + h.Address()})
	if err != nil {
		panic(err)
	}
	defer client.Close()
	c := cli.New()
	c.Append(&command.DatasetCommandBuilder{})
	viper.Set("core.endpoint", client.Config.Endpoint)
	cmd = c.Root
	return m.Run()
}
