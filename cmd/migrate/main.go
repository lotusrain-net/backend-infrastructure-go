package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lotusrain-net/backend-infrastructure-go/internal/app"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/config"
	bootstrap "github.com/lotusrain-net/backend-infrastructure-go/pkg/lifecycle"
	platformlogging "github.com/lotusrain-net/backend-infrastructure-go/pkg/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err == nil {
		direction := "up"
		if len(os.Args) > 1 {
			direction = os.Args[1]
		}
		level, levelErr := platformlogging.ParseLevel(cfg.LogLevel)
		if levelErr != nil {
			err = levelErr
		} else {
			runtime, buildErr := app.BuildMigrate(cfg, direction)
			if buildErr != nil {
				err = buildErr
			} else {
				err = bootstrap.Run(ctx, platformlogging.New(os.Stdout, level, cfg.ServiceName), cfg.ShutdownTimeout, runtime)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
