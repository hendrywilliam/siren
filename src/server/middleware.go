package server

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/lyrical/interactions"
)

func (server *Server) PingRequestMiddleware(c fiber.Ctx) error {
	i := new(interactions.Interaction)
	if err := c.Bind().JSON(i); err != nil {
		return err
	}
	if i.Type == interactions.InteractionTypePing {
		return c.JSON(interactions.InteractionResponse{
			Type: interactions.InteractionResponseTypePong,
		})
	}
	return c.Next()
}

func (server *Server) VerifyKeyMiddleware(c fiber.Ctx) error {
	pubKey := os.Getenv("DC_PUBLIC_KEY")
	if len(pubKey) == 0 {
		panic("must specify dc_public_key")
	}
	pubKeyHex, err := hex.DecodeString(pubKey)
	if err != nil {
		panic("failed to decode pub key")
	}
	body := c.BodyRaw()
	headers := c.GetReqHeaders()
	timestamp, ok := headers["X-Signature-Timestamp"]
	if !ok {
		return c.SendString("invalid timestamp signature")
	}
	signature, ok := headers["X-Signature-Ed25519"]
	if !ok {
		return c.SendString("invalid ed25519 signature")
	}
	signatureHex, err := hex.DecodeString(signature[0])
	if err != nil {
		panic("failed to decode signature")
	}
	message := bytes.Join([][]byte{[]byte(timestamp[0]), body}, []byte(""))
	isVerified := ed25519.Verify(pubKeyHex, message, signatureHex)
	if !isVerified {
		return c.Status(401).SendString("invalid request signature")
	}
	return c.Next()
}
