package server

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/siren/interactions"
)

type Server struct {
	router *fiber.App
}

func NewServer() *Server {
	return &Server{
		router: fiber.New(),
	}
}

func (server *Server) setupRouter() {
	router := fiber.New()
	router.Use("/", server.VerifyKeyMiddleware)
	router.Use("/", server.PingRequestMiddleware)
	router.Post("/interactions", func(c fiber.Ctx) error {
		req := new(interactions.Interaction)
		if err := c.Bind().JSON(req); err != nil {
			log.Error(err)
			return c.Status(http.StatusInternalServerError).SendString("internal server error")
		}
		if req.Type == interactions.InteractionTypeApplicationCommand {
			if req.Data.Name == "test" {
				return c.JSON(interactions.InteractionResponse{
					Type: interactions.InteractionResponseTypeChannelMessageWithSource,
					Data: interactions.InteractionResponseDataMessage{
						Content: "hello world",
					},
				})
			}
			log.Error("unknown command")
			return c.Status(http.StatusBadRequest).JSON("error: 'unknown request'")
		}
		log.Error("unknown interaction type")
		return c.Status(http.StatusBadRequest).JSON(("error: 'bad request'}"))
	})
	server.router = router
	return
}

func (server *Server) StartServer(ctx context.Context, addr string) {
	log.Info(fmt.Sprintf("server start at %s", os.Getenv("API_ADDRESS")))
	server.setupRouter()
	server.router.Listen(addr, fiber.ListenConfig{
		GracefulContext: ctx,
		OnShutdownSuccess: func() {
			log.Info("server stopped.")
		},
	})
}
