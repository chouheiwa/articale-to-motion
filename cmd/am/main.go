package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/chouheiwa/articale-to-motion/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.ExecuteContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
