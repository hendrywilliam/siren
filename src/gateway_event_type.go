package src

type GatewayOpcode = uint8

const (
	GatewayOpcodeDispatch                GatewayOpcode = 0
	GatewayOpcodeHeartbeat               GatewayOpcode = 1
	GatewayOpcodeIdentify                GatewayOpcode = 2
	GatewayOpcodePresenceUpdate          GatewayOpcode = 3
	GatewayOpcodeVoiceStateUpdate        GatewayOpcode = 4
	GatewayOpcodeResume                  GatewayOpcode = 6
	GatewayOpcodeReconnect               GatewayOpcode = 7
	GatewayOpcodeRequestGuildMember      GatewayOpcode = 8
	GatewayOpcodeInvalidSession          GatewayOpcode = 9
	GatewayOpcodeHello                   GatewayOpcode = 10
	GatewayOpcodeHeartbeatAck            GatewayOpcode = 11
	GatewayOpcodeRequestSoundboardSounds GatewayOpcode = 31
)

type EventName = string
type EventOpcode = uint8

type GatewayEvent struct {
	Op EventOpcode `json:"op"`
	D  interface{} `json:"d,omitempty"`
	S  uint64      `json:"s,omitempty"`
	T  EventName   `json:"t,omitempty"`
}

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
