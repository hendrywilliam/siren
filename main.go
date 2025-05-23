package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	internalLog "github.com/hendrywilliam/siren/src"
	"github.com/hendrywilliam/siren/src/gateway"
	"github.com/hendrywilliam/siren/src/utils"
	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		slog.Error("Failed to load configuration file.")
		os.Exit(1)
	}
	env := utils.LoadConfiguration()
	var logHandler slog.Handler
	logOpts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	if env.AppEnv != "production" {
		logHandler = internalLog.NewCustomHandler(os.Stdout, internalLog.CustomHandlerOpts{
			SlogOpts: logOpts,
		})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &logOpts)
	}
	logger := slog.New(logHandler)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	g := gateway.NewGateway(gateway.DiscordArguments{
		BotToken:   env.DiscordBotToken,
		BotVersion: 10,
		BotIntent: []gateway.GatewayIntent{
			gateway.GuildsIntent,
			gateway.GuildVoiceStatesIntent,
			gateway.GuildMessagesIntent,
			gateway.MessageContentIntent,
		},
		ClientID: env.DiscordClientID,
		Logger:   logger,
	})
	g.Open(ctx)
	<-ctx.Done()
}
