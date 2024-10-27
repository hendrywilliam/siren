package main

import (
	"context"

	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3/log"
	"github.com/joho/godotenv"
	"github.com/lyrical/server"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGINT,
	syscall.SIGTERM,
}

func main() {
	err := godotenv.Load("./.env")
	if err != nil {
		log.Panic("failed to load config file")
	}
	// interactions.RegisterCommands()
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	server := server.NewServer()
	addrs := os.Getenv("API_ADDRESS")
	server.StartServer(ctx, addrs)
}
