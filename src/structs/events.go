package structs

type EventName = string

const (
	EventNameInteractionCreate EventName = "INTERACTION_CREATE"
	EventNameReady             EventName = "READY"
	EventNameVoiceServerUpdate EventName = "VOICE_SERVER_UPDATE"
	EventNameVoiceStateUpdate  EventName = "VOICE_STATE_UPDATE"
)

type Event struct {
	Op EventOpcode            `json:"op"`
	D  map[string]interface{} `json:"d,omitempty"`
	S  uint64                 `json:"s,omitempty"`
	T  EventName              `json:"t,omitempty"`
}

// Send events.
type HeartbeatEvent struct {
	Op EventOpcode `json:"op"`
	D  uint64      `json:"d"`
}

type ResumeEvent struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       uint64 `json:"seq"`
}

type UpdateVoiceStateD struct {
	GuildID   string `json:"guild_id"`
	ChannelID string `json:"channel_id"`
	SelfMute  bool   `json:"self_mute"`
	SelfDeaf  bool   `json:"false"`
}

type UpdateVoiceState struct {
	Op EventOpcode       `json:"op"`
	D  UpdateVoiceStateD `json:"d"`
}

// Receive events.
type HelloEvent struct {
	HeartbeatInterval uint64 `json:"heartbeat_interval"`
}

type ReadyEvent struct {
	T  EventName      `json:"t"`
	S  uint64         `json:"s"`
	Op EventOpcode    `json:"op"`
	D  ReadyEventData `json:"d"`
}

type ReadyEventData struct {
	V                uint8       `json:"v"`
	User             interface{} `json:"user"`
	Guilds           interface{} `json:"guilds"`
	SessionID        string      `json:"session_id"`
	ResumeGatewayURL string      `json:"resume_gateway_url"`
	Shard            []uint      `json:"shard,omitempty"`
	Application      interface{} `json:"application"`
}
