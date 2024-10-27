package interactions

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
	Embeds          interface{} `json:"embeds,omitempty"`           // unimplemented.
	AllowedMentions interface{} `json:"allowed_mentions,omitempty"` // unimplemented.
	Components      interface{} `json:"components,omitempty"`       // unimplemented.
	Attachments     interface{} `json:"attachments,omitempty"`      // unimplemented.
	Poll            interface{} `json:"poll,omitempty"`             // unimplemented.
}

type InteractionResponse struct {
	Type InteractionResponseType        `json:"type"`
	Data InteractionResponseDataMessage `json:"data,omitempty"`
}
