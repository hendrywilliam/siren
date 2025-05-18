package structs

import (
	"time"
)

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

type VoiceStateUpdateData struct {
	Member                  Member    `json:"member"`
	UserID                  string    `json:"user_id"`
	Suppress                bool      `json:"suppress"`
	SessionID               string    `json:"session_id"`
	SelfVideo               bool      `json:"self_video"`
	SelfMute                bool      `json:"self_mute"`
	SelfDeaf                bool      `json:"self_deaf"`
	RequestToSpeakTimestamp time.Time `json:"request_to_speak_timestamp,omitempty"`
	Mute                    bool      `json:"mute"`
	GuildID                 string    `json:"guild_id"`
	Deaf                    bool      `json:"deaf"`
	ChannelID               string    `json:"channel_id"`
}

type VoiceServerUpdateData struct {
	Token    string `json:"token"`
	GuildID  string `json:"guild_id"`
	Endpoint string `json:"endpoint"`
}
