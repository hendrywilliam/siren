package structs

type InteractionType = uint8

const (
	InteractionTypePing                           InteractionType = 1
	InteractionTypeApplicationCommand             InteractionType = 2
	InteractionTypeMessageComponent               InteractionType = 3
	InteractionTypeApplicationCommandAutocomplete InteractionType = 4
	InteractionTypeModalSubmit                    InteractionType = 5
)

type InteractionContextType = uint8

const (
	InteractionContextTypeGuild InteractionContextType = 0
	InteractionContextTypeBotDM InteractionContextType = 1
	InteractionPrivateChannel   InteractionContextType = 2
)

type InteractionApplicationCommandData struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     uint        `json:"type"`
	Resolved interface{} `json:"resolved,omitempty"`
	Options  interface{} `json:"options,omitempty"`
	GuildID  string      `json:"guild_id,omitempty"`
	TargetID string      `json:"target_id,omitempty"`
}

type Interaction struct {
	ID                           string                            `json:"id"`
	ApplicationID                string                            `json:"application_id"`
	Type                         InteractionType                   `json:"type"`
	Data                         InteractionApplicationCommandData `json:"data,omitempty"`
	GuildID                      string                            `json:"guild_id,omitempty"`
	Token                        string                            `json:"token"`
	Version                      uint                              `json:"version"`
	Context                      InteractionContextType            `json:"context,omitempty"`
	Guild                        interface{}                       `json:"guild,omitempty"`
	Channel                      interface{}                       `json:"channel,omitempty"`
	Member                       Member                            `json:"member,omitempty"`
	User                         User                              `json:"user,omitempty"`
	Message                      interface{}                       `json:"message,omitempty"`
	AppPermissions               string                            `json:"app_permissions,omitempty"`
	Locale                       string                            `json:"locale,omitempty"`
	GuildLocale                  string                            `json:"guild_locale,omitempty"`
	Entitlements                 []interface{}                     `json:"entitlements,omitempty"`
	AuthorizingIntegrationOwners interface{}                       `json:"authorizing_integration_owners,omitempty"`
}

type InteractionResponseType = uint

const (
	InteractionResponseTypePong                                 InteractionResponseType = 1
	InteractionResponseTypeChannelMessageWithSource             InteractionResponseType = 4
	InteractionResponseTypeDeferredChannelMessageWithSource     InteractionResponseType = 5
	InteractionResponseTypeDeferredUpdateMessage                InteractionResponseType = 6
	InteractionResponseTypeUpdateMessage                        InteractionResponseType = 7
	InteractionResponseTypeApplicationCommandAutoCompleteResult InteractionResponseType = 8
	InteractionResponseTypeModal                                InteractionResponseType = 9
	InteractionResponseTypePremiumRequired                      InteractionResponseType = 10
	InteractionResponseTypeLaunchActivity                       InteractionResponseType = 12
)

type InteractionResponseDataMessage struct {
	Tts             bool        `json:"tts,omitempty"`
	Content         string      `json:"content,omitempty"`
	Flags           uint        `json:"flags,omitempty"`
	Embeds          interface{} `json:"embeds,omitempty"`
	AllowedMentions interface{} `json:"allowed_mentions,omitempty"`
	Components      interface{} `json:"components,omitempty"`
	Attachments     interface{} `json:"attachments,omitempty"`
	Poll            interface{} `json:"poll,omitempty"`
}

type InteractionResponse struct {
	Type InteractionResponseType        `json:"type"`
	Data InteractionResponseDataMessage `json:"data,omitempty"`
}
