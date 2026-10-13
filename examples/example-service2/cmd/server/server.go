package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/FlyTOmeLight/server-go/app"
	"github.com/FlyTOmeLight/server-go/httpintegration"

	"github.com/FlyTOmeLight/server-go/examples/example-service2/service"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, app.Options{Setup: httpintegration.Install}, service.Build); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
