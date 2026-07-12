package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"backend-infrastructure-go/internal/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := bootstrap.Execute(ctx, os.Stdout, "migrate"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
