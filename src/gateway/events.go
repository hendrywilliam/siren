package gateway

type EventOpcode = uint64

type Event struct {
	Op EventOpcode `json:"op"`
	D  interface{} `json:"d,omitempty"`
	S  uint64      `json:"s,omitempty"`
	T  string      `json:"t,omitempty"`
}

type SendOpcode = uint64

const (
	HeartbeatSendOpcode               SendOpcode = 1
	IdentifySendOpcode                SendOpcode = 2
	PresenceUpdateSendOpcode          SendOpcode = 3
	VoiceStateSendOpcode              SendOpcode = 4
	ResumeSendOpcode                  SendOpcode = 6
	RequestGuildMembersSendOpcode     SendOpcode = 8
	RequestSoundboardSoundsSendOpcode SendOpcode = 31
)

type HelloEvent struct {
	Op EventOpcode `json:"op"`
	D  struct {
		HeartbeatInterval uint `json:"heartbeat_interval"`
	} `json:"d"`
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
