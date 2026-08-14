package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jyysy/backend-infrastructure-go/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Execute(ctx, os.Stdout, "api"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
