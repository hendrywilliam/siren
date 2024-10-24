package src

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Cmd struct {
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Type             int64   `json:"type"`
	IntegrationTypes []int64 `json:"integration_types"`
	Contexts         []int64 `json:"contexts"`
}

func InstallCmds(ctx context.Context) {
	botToken := os.Getenv("BOT_TOKEN")
	if len(botToken) == 0 {
		log.Fatal("bot token is not provided")
	}
	endpoint := fmt.Sprintf("https://discord.com/api/%s/applications/%s/commands", os.Getenv("APPS_VERSION"), os.Getenv("APPLICATION_ID"))
	rb, err := json.Marshal([]Cmd{
		{
			Name:             "test",
			Description:      "Basic command",
			Type:             1,
			IntegrationTypes: []int64{0, 1},
			Contexts:         []int64{0, 1, 2},
		},
	})
	if err != nil {
		log.Fatal("failed to marshalling")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewBuffer(rb))
	request.Header.Set("Authorization", fmt.Sprintf("Bot %s", os.Getenv("BOT_TOKEN")))
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("User-Agent", "DiscordBot (https://github.com/discord/discord-example-app, 1.0.0)")

	httpClient := &http.Client{}
	response, err := httpClient.Do(request)
	if err != nil {
		log.Fatal("failed to register commands:", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	fmt.Println("response body", string(body))
}
