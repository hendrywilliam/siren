package main

import (
	"context"

	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/siren/gateway"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGINT,
	syscall.SIGTERM,
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
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
