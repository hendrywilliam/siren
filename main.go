package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hendrywilliam/siren/src"
	"github.com/joho/godotenv"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGINT,
	syscall.SIGTERM,
}

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("failed to load config file")
	}
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	g := src.NewGateway(ctx)
	err = g.Open()
	if err != nil {
		stop()
	}
	<-ctx.Done()
}
