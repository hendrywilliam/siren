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
	Resolved interface{} `json:"resolved,omitempty"`  // unimplemented.
	Options  interface{} `json:"options,omitempty"`   // unimplemented.
	GuildID  string      `json:"guild_id,omitempty"`  // unimplemented.
	TargetID string      `json:"target_id,omitempty"` // unimplemented.
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
	Guild                        interface{}                       `json:"guild,omitempty"`                          // unimplemented.
	Channel                      interface{}                       `json:"channel,omitempty"`                        // unimplemented.
	Member                       struct{}                          `json:"member,omitempty"`                         // unimplemented.
	User                         interface{}                       `json:"user,omitempty"`                           // unimplemented.
	Message                      interface{}                       `json:"message,omitempty"`                        // unimplemented.
	AppPermissions               string                            `json:"app_permissions,omitempty"`                // unimplemented.
	Locale                       string                            `json:"locale,omitempty"`                         // unimplemented.
	GuildLocale                  string                            `json:"guild_locale,omitempty"`                   // unimplemented.
	Entitlements                 []interface{}                     `json:"entitlements,omitempty"`                   // unimplemented.
	AuthorizingIntegrationOwners interface{}                       `json:"authorizing_integration_owners,omitempty"` // unimplemented.
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
