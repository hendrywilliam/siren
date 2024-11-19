package src

import (
	"time"
)

type VoiceOpcode = uint8

const (
	VoiceOpcodeIdentify           VoiceOpcode = 0
	VoiceOpcodeSelectProtocol     VoiceOpcode = 1
	VoiceOpcodeReady              VoiceOpcode = 2
	VoiceOpcodeHeartbeat          VoiceOpcode = 3
	VoiceOpcodeSessionDescription VoiceOpcode = 4
	VoiceOpcodeSpeaking           VoiceOpcode = 5
	VoiceOpcodeHeartbeatAck       VoiceOpcode = 6
	VoiceOpcodeResume             VoiceOpcode = 7
	VoiceOpcodeHello              VoiceOpcode = 8
	VoiceOpcodeResumed            VoiceOpcode = 9
	VoiceOpcodeClientsConnect     VoiceOpcode = 11
	VoiceOpcodeClientDisconnect   VoiceOpcode = 13

	// dave opcodes
	VoiceOpcodeDAVEPrepareTransition        VoiceOpcode = 21
	VoiceOpcodeDAVEExecuteTransition        VoiceOpcode = 22
	VoiceOpcodeDAVETransitionReady          VoiceOpcode = 23
	VoiceOpcodeDAVEPrepareEpoch             VoiceOpcode = 24
	VoiceOpcodeDAVEMLSExternalSender        VoiceOpcode = 25
	VoiceOpcodeDAVEMLSKeyPackage            VoiceOpcode = 26
	VoiceOpcodeDAVEMLSProposals             VoiceOpcode = 27
	VoiceOpcodeDAVECommitWelcome            VoiceOpcode = 28
	VoiceOpcodeDAVEAnnounceCommitTransition VoiceOpcode = 29
	VoiceOpcodeDAVEMLSWelcome               VoiceOpcode = 30
	VoiceOpcodeDAVEMLSInvalidCommitWelcome  VoiceOpcode = 31
)

// voice close event codes
type VoiceCloseCode = int

const (
	VoiceCloseEventCodesUnknownOpcode         VoiceCloseCode = 4001
	VoiceCloseEventCodesFailedToDecodePayload VoiceCloseCode = 4002
	VoiceCloseEventCodesNotAuthenticated      VoiceCloseCode = 4003
	VoiceCloseEventCodesAuthenticationFailed  VoiceCloseCode = 4004
	VoiceCloseEventCodesAlreadyAuthenticated  VoiceCloseCode = 4005
	VoiceCloseEventCodesSessionNoLongerValid  VoiceCloseCode = 4006
	VoiceCloseEventCodesSessionTimeout        VoiceCloseCode = 4009
	VoiceCloseEventCodesServerNotFound        VoiceCloseCode = 4011
	VoiceCloseEventCodesUnknownProtocol       VoiceCloseCode = 4012
	VoiceCloseEventCodesDisconnected          VoiceCloseCode = 4014
	VoiceCloseEventCodesVoiceServerCrashed    VoiceCloseCode = 4015
	VoiceCloseEventCodesUnknownEncryptionMode VoiceCloseCode = 4016
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
