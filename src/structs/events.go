package structs

import (
	"encoding/json"
	"log/slog"
)

type EventName = string
type EventOpcode = int

const (
	EventNameInteractionCreate EventName = "INTERACTION_CREATE"
	EventNameReady             EventName = "READY"
	EventNameVoiceServerUpdate EventName = "VOICE_SERVER_UPDATE"
	EventNameVoiceStateUpdate  EventName = "VOICE_STATE_UPDATE"
)

// All events are encapsulated in RawEvent/Event.
// RawEvent has RawMessage field to delay computation.
// Suitable for any event with "Dispatch" opcode.
type RawEvent struct {
	Op     EventOpcode     `json:"op"`
	D      json.RawMessage `json:"d,omitempty"`
	S      uint64          `json:"s,omitempty"`
	T      EventName       `json:"t,omitempty"`
	Struct any             // Actual D struct.
}

func (re *RawEvent) LogValue() slog.Value {
	return slog.GroupValue(slog.Int("op_code", re.Op),
		slog.Any("event_data", re.D),
		slog.Uint64("sequence", re.S),
		slog.String("event_name", re.T))
}

type Event struct {
	Op EventOpcode `json:"op"`
	D  interface{} `json:"d,omitempty"`
	S  uint64      `json:"s,omitempty"`
	T  EventName   `json:"t,omitempty"`
}

func (e *Event) LogValue() slog.Value {
	return slog.GroupValue(slog.Int("op_code", e.Op),
		slog.Any("event_data", e.D),
		slog.Uint64("sequence", e.S),
		slog.String("event_name", e.T))
}

type ReadyEvent struct {
	V                int         `json:"v"`
	User             interface{} `json:"user"`
	Guilds           interface{} `json:"guilds"`
	SessionID        string      `json:"session_id"`
	ResumeGatewayURL string      `json:"resume_gateway_url"`
	Shard            []uint      `json:"shard,omitempty"`
	Application      interface{} `json:"application"`
}

type IdentifyEvent struct {
	Token          string                  `json:"token"`
	Properties     IdentifyEventProperties `json:"properties"`
	Intents        int                     `json:"intents"`
	Compress       bool                    `json:"compress,omitempty"`
	LargeThreshold uint8                   `json:"large_threshold"`
	Shard          any                     `json:"shard,omitempty"`
	Presence       any                     `json:"presence,omitempty"`
}

// Represent a message sent in a channel within Discord.
// https://discord.com/developers/docs/resources/message
type MessageEvent struct {
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

type IdentifyEventProperties struct {
	Os      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

type HelloEvent struct {
	HeartbeatInterval uint `json:"heartbeat_interval"`
}

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
