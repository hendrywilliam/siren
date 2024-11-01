package gateway

type EventName = string

const (
	InteractionCreate EventName = "INTERACT"
)

type EventOpcode = uint8

type Event struct {
	Op EventOpcode `json:"op"`
	D  interface{} `json:"d,omitempty"`
	S  uint64      `json:"s,omitempty"`
	T  EventName   `json:"t,omitempty"`
}

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

type HelloEventD struct {
	HeartbeatInterval uint64 `json:"heartbeat_interval"`
}

type HelloEvent struct {
	Op EventOpcode `json:"op"`
	D  HelloEventD `json:"d"`
}

type IdentifyEventDProperties struct {
	Os      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

type IdentifyEventD struct {
	Token      string                   `json:"token"`
	Intents    uint64                   `json:"intents"`
	Properties IdentifyEventDProperties `json:"properties"`
}

type IdentifyEvent struct {
	Op EventOpcode    `json:"op"`
	D  IdentifyEventD `json:"d"`
}
