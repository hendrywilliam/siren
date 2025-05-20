package structs

// Represent a message sent in a channel within Discord.
// https://discord.com/developers/docs/resources/message

type Message struct {
	ID                   string      `json:"id"`
	ChannelID            string      `json:"channel_id"`
	Author               User        `json:"author"`
	Content              string      `json:"content"`
	Timestamp            string      `json:"timestamp"`
	EditedTimestamp      string      `json:"edited_timestamp,omitempty"`
	TTS                  bool        `json:"tts"`
	MentionEveryone      bool        `json:"mention_everyone"`
	Nonce                string      `json:"nonce"`
	Type                 int         `json:"type"`
	Interaction          Interaction `json:"interaction"`
	Mentions             any         // unimplemented
	MentionRoles         any         // unimplemented
	MentionChannels      any         // unimplemented
	Attachments          any         // unimplemented
	Embeds               any         // unimplemented
	Reactions            any         // unimplemented
	Pinned               any         // unimplemented
	WebhookID            any         // unimplemented
	Activity             any         // unimplemented
	Application          any         // unimplemented
	ApplicationID        any         // unimplemented
	Flags                any         // unimplemented
	MessageReference     any         // unimplemented
	MessageSnapshots     any         // unimplemented
	ReferencedMessage    any         // unimplemented
	InteractionMetadata  any         // unimplemented
	Thread               any         // unimplemented
	Components           any         // unimplemented
	StickerItems         any         // unimplemented
	Position             any         // unimplemented
	RoleSubscriptionData any         // unimplemented
	Resolved             any         // unimplemented
	Poll                 any         // unimplemented
	Call                 any         // unimplemented
}
