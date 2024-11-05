package main

import (
	"context"

	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/siren/src/gateway"
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
	// interactions.RegisterCommands()
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	// server := server.NewServer()
	addrs, ok := os.LookupEnv("API_ADDRESS")
	if !ok || len(addrs) == 0 {
		panic("api_address is not provided")
	}
	// go server.StartServer(ctx, addrs)
	go gateway.StartGateway(ctx)
	<-ctx.Done()
}
