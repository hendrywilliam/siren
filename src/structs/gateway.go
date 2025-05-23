package structs

type GatewayEventData interface{}

type HelloEventData struct {
	HeartbeatInterval uint `json:"heartbeat_interval"`
}

type HeartbeatEventData = uint64

type IdentifyEventDProperties struct {
	Os      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

type IdentifyEventData struct {
	Token      string                   `json:"token"`
	Intents    uint64                   `json:"intents"`
	Properties IdentifyEventDProperties `json:"properties"`
}

type GatewayVoiceState struct {
	GuildID   string `json:"guild_id"`
	ChannelID string `json:"channel_id"`
	SelfMute  bool   `json:"self_mute"`
	SelfDeaf  bool   `json:"self_deaf"`
}
