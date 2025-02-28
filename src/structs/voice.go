package structs

import (
	"time"
)

// voice state
type VoiceState struct {
	GuildID                 string      `json:"guild_id"`
	ChannelID               string      `json:"channel_id"`
	UserID                  string      `json:"user_id"`
	Member                  interface{} `json:"member,omitempty"`
	SessionID               string      `json:"session_id"`
	Deaf                    bool        `json:"deaf"`
	Mute                    bool        `json:"mute"`
	SelfDeaf                bool        `json:"self_deaf"`
	SelfMute                bool        `json:"self_mute"`
	SelfStream              bool        `json:"self_stream"`
	SelfVideo               bool        `json:"self_video"`
	Suppress                bool        `json:"suppress"`
	RequestToSpeakTimestamp time.Time   `json:"request_to_speak_timestamp"`
}

// identify payload
type VoiceIdentify struct {
	ServerId  string `json:"server_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Token     string `json:"token"`
}

type VoiceReadyStreams struct {
	Active  bool   `json:"active"`
	Quality uint   `json:"quality"`
	RID     string `json:"rid"`
	RTXSSRC uint16 `json:"rtx_ssrc"`
	SSRC    uint32 `json:""`
}

type VoiceClientsConnect struct {
	UserIds []string `json:"user_ids"`
}

type VoiceResume struct {
	ServerID            string
	SessionID           string
	Token               string
	SequenceAcknowledge uint64
}

type VoiceReady struct {
	Experiments []string          `json:"experiments"`
	SSRC        uint32            `json:"ssrc"`
	IP          string            `json:"ip"`
	Port        uint16            `json:"port"`
	Modes       []string          `json:"modes"`
	Streams     VoiceReadyStreams `json:"stream"`
}

type VoiceIPDiscovery struct {
	Type    uint16
	Length  uint16
	SSRC    uint32
	Address [64]byte
	Port    uint16
}

type SelectProtocolData struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
	Mode    string `json:"mode"`
}

type SelectProtocol struct {
	Protocol string             `json:"protocol"`
	Data     SelectProtocolData `json:"data"`
}

type SessionDescription struct {
	AudioCodec          string   `json:"audio_codec"`
	DaveProtocolVersion uint8    `json:"dave_protocol_version"`
	MediaSessionID      string   `json:"media_session_id"`
	Mode                string   `json:"mode"`
	SecretKey           [32]byte `json:"secret_key"`
	SecureFramesVersion uint8    `json:"secure_frames_version"`
	VideoCodec          string   `json:"video_codec"`
}

type HeartbeatData struct {
	T      int64  `json:"t"`
	SeqAck uint64 `json:"seq_ack"` // needed in v8 or greater.
}

type ClientsConnect struct {
	UsersID []string `json:"users_id"`
}

type Speaking struct {
	Speaking uint   `json:"speaking"`
	Delay    uint   `json:"delay"`
	SSRC     uint32 `json:"ssrc"`
}
