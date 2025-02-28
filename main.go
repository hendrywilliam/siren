package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hendrywilliam/siren/src"
	internalLog "github.com/hendrywilliam/siren/src"
	"github.com/hendrywilliam/siren/src/utils"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Error("Failed to load configuration file.")
		os.Exit(1)
	}
	cfg := utils.LoadConfiguration()
	var logHandler slog.Handler
	logOpts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	if cfg.AppEnv != "production" {
		logHandler = internalLog.NewCustomHandler(os.Stdout, internalLog.CustomHandlerOpts{
			SlogOpts: logOpts,
		})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &logOpts)
	}
	logger := slog.New(logHandler)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	g := src.NewGateway(src.GatewayArguments{
		Config: cfg,
		Logger: logger,
	})
	g.Open(ctx)
	<-ctx.Done()
}
