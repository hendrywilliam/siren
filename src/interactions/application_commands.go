package interactions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v3/log"
)

type AppCmdType = uint8

const (
	AppCmdTypeChatInput  AppCmdType = 1
	AppCmdTypeUser       AppCmdType = 2
	AppCmdTypeMessage    AppCmdType = 3
	AppPrimaryEntryPoint AppCmdType = 4
)

type AppCmdIntegrationType = uint8

const (
	AppIntegrationTypeGuildInstall AppCmdIntegrationType = 0
	AppIntegrationTypeUserInstall  AppCmdIntegrationType = 1
)

type AppCmdInteractionCtxType = uint8

const (
	AppInteractionContextTypeGuild          AppCmdInteractionCtxType = 0
	AppInteractionContextTypeBotDM          AppCmdInteractionCtxType = 1
	AppInteractionContextTypePrivateChannel AppCmdInteractionCtxType = 2
)

type AppCmd struct {
	ID                      string                     `json:"id,omitempty"`
	Type                    AppCmdType                 `json:"type,omitempty"`
	ApplicationID           string                     `json:"application_id,omitempty"`
	GuildId                 string                     `json:"guild_id,omitempty"`
	Name                    string                     `json:"name"`
	NameLocalization        interface{}                `json:"name_localization,omitempty"`
	Description             string                     `json:"description"`
	DescriptionLocalization string                     `json:"description_localizations,omitempty"`
	Options                 interface{}                `json:"options,omitempty"`
	IntegrationTypes        []AppCmdIntegrationType    `json:"integration_types"`
	Contexts                []AppCmdInteractionCtxType `json:"contexts"`
	Nsfw                    bool                       `json:"nsfw,omitempty"`
	Version                 string                     `json:"version,omitempty"`
}

func RegisterCommands() {
	botToken := os.Getenv("DC_BOT_TOKEN")
	if len(botToken) == 0 {
		log.Fatal(fmt.Errorf("no bot token provided"))
	}
	dcAppVersion, ok := os.LookupEnv("DC_API_VERSION")
	if !ok || len(dcAppVersion) == 0 {
		log.Fatal(fmt.Errorf("dc_api_version is not provided"))
	}
	dcApplicationId := os.Getenv("DC_APPLICATION_ID")
	if !ok || len(dcApplicationId) == 0 {
		log.Fatal(fmt.Errorf("dc_application_id is not provided"))
	}
	endpoint := fmt.Sprintf("https://discord.com/api/%s/applications/%s/commands", dcAppVersion, dcApplicationId)
	commands := []AppCmd{
		{
			Name:             "test",
			Description:      "test command",
			Type:             AppCmdTypeChatInput,
			IntegrationTypes: []uint8{AppIntegrationTypeGuildInstall, AppIntegrationTypeUserInstall},
			Contexts:         []uint8{AppInteractionContextTypeGuild, AppInteractionContextTypePrivateChannel, AppInteractionContextTypeBotDM},
		},
	}
	rb, err := json.Marshal(commands)
	if err != nil {
		log.Fatal("failed to marshall.")
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPut, endpoint, bytes.NewBuffer(rb))
	request.Header.Set("Authorization", fmt.Sprintf("Bot %s", botToken))
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("User-Agent", "DiscordBot (https://github.com/discord/discord-example-app, 1.0.0)")

	httpClient := &http.Client{}
	response, err := httpClient.Do(request)
	if err != nil {
		log.Fatal("failed to get a response from discord: ")
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	log.Debug(string(body))
}
