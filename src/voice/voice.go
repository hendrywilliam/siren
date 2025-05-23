package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hendrywilliam/siren/src/audio"
	"github.com/hendrywilliam/siren/src/audiosender"
	"github.com/hendrywilliam/siren/src/structs"
)

type VoiceGatewayStatus = string

const (
	StatusReady        VoiceGatewayStatus = "READY"
	StatusDisconnected VoiceGatewayStatus = "DISCONNECTED"
)

var (
	SpeakingModeMicrophone = 1 << 0
	SpeakingModeSoundshare = 1 << 1
	SpeakingModePriority   = 1 << 2
)

type VoiceOpcode = int

const (
	OpcodeIdentify           VoiceOpcode = 0
	OpcodeSelectProtocol     VoiceOpcode = 1
	OpcodeReady              VoiceOpcode = 2
	OpcodeHeartbeat          VoiceOpcode = 3
	OpcodeSessionDescription VoiceOpcode = 4
	OpcodeSpeaking           VoiceOpcode = 5
	OpcodeHeartbeatAck       VoiceOpcode = 6
	OpcodeResume             VoiceOpcode = 7
	OpcodeHello              VoiceOpcode = 8
	OpcodeResumed            VoiceOpcode = 9
	OpcodeClientsConnect     VoiceOpcode = 11
	OpcodeClientDisconnect   VoiceOpcode = 13

	// Dave opcodes
	DAVEPrepareTransition        VoiceOpcode = 21
	DAVEExecuteTransition        VoiceOpcode = 22
	DAVETransitionReady          VoiceOpcode = 23
	DAVEPrepareEpoch             VoiceOpcode = 24
	DAVEMLSExternalSender        VoiceOpcode = 25
	DAVEMLSKeyPackage            VoiceOpcode = 26
	DAVEMLSProposals             VoiceOpcode = 27
	DAVECommitWelcome            VoiceOpcode = 28
	DAVEAnnounceCommitTransition VoiceOpcode = 29
	DAVEMLSWelcome               VoiceOpcode = 30
	DAVEMLSInvalidCommitWelcome  VoiceOpcode = 31
)

// voice close event codes
type VoiceCloseCode = int

const (
	UnknownOpcode        VoiceCloseCode = 4001
	FailedToDecode       VoiceCloseCode = 4002
	NotAuthenticated     VoiceCloseCode = 4003
	AuthenticationFailed VoiceCloseCode = 4004
	AlreadyAuthenticated VoiceCloseCode = 4005
	SessionInvalid       VoiceCloseCode = 4006
	SessionTimeout       VoiceCloseCode = 4009
	ServerNotFound       VoiceCloseCode = 4011
	UnknownProtocol      VoiceCloseCode = 4012
	Disconnected         VoiceCloseCode = 4014
	ServerCrashed        VoiceCloseCode = 4015
	UnknownEncryption    VoiceCloseCode = 4016
)

var (
	ErrUnrecognizedEvent = errors.New("unrecognized event")
)

type Voice struct {
	rwlock     sync.RWMutex
	wsDialer   *websocket.Dialer
	wsConn     *websocket.Conn
	log        *slog.Logger
	ctx        context.Context
	cancelFunc context.CancelFunc

	status     VoiceGatewayStatus
	botVersion uint
	sequence   atomic.Uint64

	heartbeatTicker *time.Ticker

	udpConn        *net.UDPConn
	port           uint16
	ssrc           uint32
	ip             string
	encryptionMode string

	// Voice identifier
	SessionID       string
	ServerID        string // Guild ID
	UserID          string
	VoiceGatewayURL string
	Token           string

	// Audio state, APIs, data.
	secretKeys  [32]byte
	audio       *audio.Audio
	audioSender *audiosender.AudioSender

	audioCtx        context.Context
	audioCancelFunc context.CancelFunc

	audioDataChan   chan []byte
	audioIsFinished chan bool
}

type NewVoiceArguments struct {
	SessionID  string
	BotVersion uint
	ServerID   string
	UserID     string

	Log *slog.Logger
}

func NewVoice(args NewVoiceArguments) *Voice {
	return &Voice{
		wsDialer:        websocket.DefaultDialer,
		status:          StatusDisconnected,
		log:             args.Log.With("voice_id", fmt.Sprintf("voice_%s", args.SessionID)),
		botVersion:      args.BotVersion,
		SessionID:       args.SessionID,
		UserID:          args.UserID,
		ServerID:        args.ServerID,
		audio:           &audio.Audio{},
		audioSender:     &audiosender.AudioSender{},
		audioDataChan:   make(chan []byte),
		audioIsFinished: make(chan bool),
	}
}

func (v *Voice) Open(ctx context.Context) error {
	return v.open(ctx)
}

func (v *Voice) open(ctx context.Context) error {
	var err error
	v.ctx, v.cancelFunc = context.WithCancel(ctx)
	url := url.URL{
		Scheme:   "wss",
		Host:     v.VoiceGatewayURL,
		RawQuery: fmt.Sprintf("v=%d", v.botVersion),
	}
	v.wsConn, _, err = v.wsDialer.DialContext(v.ctx, url.String(), nil)
	if err != nil {
		v.log.Error(err.Error())
		return err
	}
	identifyEvent := &structs.Event{
		Op: OpcodeIdentify,
		D: structs.VoiceIdentify{
			ServerId:  v.ServerID,
			UserID:    v.UserID,
			SessionID: v.SessionID,
			Token:     v.Token,
		},
	}
	data, err := json.Marshal(identifyEvent)
	if err != nil {
		return err
	}

	err = v.sendEvent(websocket.TextMessage, data)
	if err != nil {
		v.log.Error(err.Error())
		return err
	}
	v.log.Info("identify event sent.")

	e := &structs.RawEvent{}
	err = v.wsConn.ReadJSON(e)
	if err != nil {
		v.log.Error(err.Error())
		return err
	}

	v.log.Info("event", "incoming_event", e)

	// init heartbeating process
	if e.Op == OpcodeHello {
		d := &structs.VoiceHello{}
		if err := json.Unmarshal(e.D, d); err != nil {
			return err
		}
		go v.heartbeating(time.Duration(d.HeartbeatInterval))
	}

	e = &structs.RawEvent{}
	err = v.wsConn.ReadJSON(e)
	if err != nil {
		return err
	}

	v.log.Info("event", "incoming_event", e)

	if e.Op == OpcodeReady {
		readyEvent := &structs.VoiceReady{}
		if err := json.Unmarshal(e.D, readyEvent); err != nil {
			return err
		}
		v.status = StatusReady
		v.ip = readyEvent.IP
		v.port = readyEvent.Port
		v.encryptionMode = "aead_xchacha20_poly1305_rtpsize"
		v.ssrc = readyEvent.SSRC

		go v.listen(v.wsConn)

		// open udp conn.
		err = v.dialUDP(readyEvent.IP, readyEvent.Port)
		if err != nil {
			v.log.Error(err.Error())
			return err
		}

	}
	return nil
}

func (v *Voice) listen(conn *websocket.Conn) {
	for {
		select {
		case <-v.ctx.Done():
			return
		default:
			v.rwlock.Lock()
			same := v.wsConn == conn
			v.rwlock.Unlock()
			if !same {
				return
			}
			messageType, message, err := conn.ReadMessage()
			v.log.Info("event", "incoming_event", message)
			if err != nil {
				v.log.Error(err.Error())
				// @todo
				panic(err)
			}
			_, err = v.acceptEvent(messageType, message)
			if err != nil {
				v.log.Error(err.Error())
			}
		}
	}
}

func (v *Voice) acceptEvent(messageType int, rawMessage []byte) (*structs.RawEvent, error) {
	var err error
	reader := bytes.NewBuffer(rawMessage)

	e := &structs.RawEvent{}
	decoder := json.NewDecoder(reader)
	if err = decoder.Decode(&e); err != nil {
		return e, err
	}

	switch e.Op {
	case OpcodeHeartbeatAck:
		v.log.Info("event", "heartbeat_acknowledge", e)
		return e, nil

	case OpcodeSessionDescription:
		v.log.Info("event", "session_description", e)

		sessionDescriptionEvent := &structs.SessionDescription{}
		if err := json.Unmarshal(e.D, sessionDescriptionEvent); err != nil {
			return nil, err
		}

		// Get secret_keys to encrypt data and encryption mode.
		v.secretKeys = sessionDescriptionEvent.SecretKey
		v.encryptionMode = sessionDescriptionEvent.Mode

		// We need to send speaking event first.
		// Then we can start sending encrypted audio data.
		speakingEvent := &structs.Event{
			Op: OpcodeSpeaking,
			D: &structs.Speaking{
				Speaking: SpeakingModeMicrophone,
				Delay:    0,
				SSRC:     v.ssrc,
			},
		}

		v.log.Info("event", "speaking_event", speakingEvent)

		var err error
		data, err := json.Marshal(speakingEvent)
		if err != nil {
			return nil, err
		}
		err = v.sendEvent(websocket.BinaryMessage, data)

		if err != nil {
			return nil, err
		}

		v.audioCtx, v.audioCancelFunc = context.WithCancel(v.ctx)
		go v.audio.Encode(v.audioCtx, "sirens.mp3", v.audioDataChan, v.audioIsFinished)
		go v.audioSender.Send(v.audioCtx, v.udpConn, v.secretKeys, v.audioDataChan, v.audioIsFinished)

		return e, nil
	default:
		v.log.Info("event", "any", e)
		return e, nil
	}
}

func (v *Voice) close() {
	if v.heartbeatTicker != nil {
		v.heartbeatTicker.Stop()
		v.heartbeatTicker = nil
	}
	v.status = StatusDisconnected
	v.cancelFunc()
	v.wsConn.Close()
	v.log.Info("connection closed.")
	return
}

func (v *Voice) heartbeating(dur time.Duration) error {
	v.heartbeatTicker = time.NewTicker(dur * time.Millisecond)
	for {
		select {
		case <-v.ctx.Done():
			v.heartbeatTicker.Stop()
			v.log.Info("heartbeating stopped.")
			return nil
		case <-v.heartbeatTicker.C:
			seq := v.sequence.Load()
			heartbeatEvent := &structs.Event{
				Op: OpcodeHeartbeat,
				D: &structs.VoiceHeartbeat{
					T:      v.nonce(),
					SeqAck: seq,
				},
			}
			data, err := json.Marshal(heartbeatEvent)
			if err != nil {
				return err
			}
			err = v.sendEvent(websocket.BinaryMessage, data)
			if err != nil {
				v.log.Error(err.Error())
				return err
			}
			v.log.Info("heartbeat event sent.")
		}
	}
}

func (v *Voice) nonce() int64 {
	return time.Now().UnixMilli()
}

func (v *Voice) sendEvent(messageType int, data []byte) error {
	v.rwlock.Lock()
	defer v.rwlock.Unlock()
	return v.wsConn.WriteMessage(messageType, data)
}

func (v *Voice) sendIPDiscovery() error {
	var packet []byte
	packet = binary.BigEndian.AppendUint16(packet, uint16(0x1))
	packet = binary.BigEndian.AppendUint16(packet, uint16(70))
	packet = binary.BigEndian.AppendUint32(packet, v.ssrc)
	var address [64]byte
	copy(address[:], v.ip)
	packet = append(packet, address[:]...)
	packet = binary.BigEndian.AppendUint16(packet, v.port)
	_, err := v.udpConn.Write(packet)
	if err != nil {
		return err
	}

	b := make([]byte, 100)
	_, err = v.udpConn.Read(b[:])
	if err != nil {
		return err
	}

	var ipAddrBuilder strings.Builder

	// Should find a better way to get the ip address?
	for i := 15; i < 30; i++ {
		if b[i] == 0 {
			break
		}
		ipAddrBuilder.Write(b[i : i+1])
	}
	ipAddr := ipAddrBuilder.String()
	port := binary.BigEndian.Uint16(b[72:74])
	fmt.Println(ipAddr, port)
	return v.sendSelectProtocol(ipAddr, port)
}

func (v *Voice) sendSelectProtocol(ipAddr string, port uint16) error {
	e := &structs.Event{
		Op: OpcodeSelectProtocol,
		D: &structs.SelectProtocol{
			Protocol: "udp",
			Data: structs.SelectProtocolData{
				Address: ipAddr,
				Port:    port,
				Mode:    v.encryptionMode,
			},
		},
	}
	data, err := json.Marshal(e)
	if err != nil {
		v.log.Error(err.Error())
		return err
	}
	err = v.sendEvent(websocket.BinaryMessage, data)
	if err != nil {
		v.log.Error(err.Error())
		return err
	}
	v.log.Info("select protocol event sent.")
	return nil
}

func (v *Voice) dialUDP(ip string, port uint16) error {
	var err error
	udpAdd, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%v", ip, port))
	if err != nil {
		v.log.Error(err.Error())
		return err
	}
	v.udpConn, err = net.DialUDP("udp", nil, udpAdd)
	if err != nil {
		v.log.Error(err.Error())
		return err
	}
	err = v.sendIPDiscovery()
	if err != nil {
		v.log.Error(err.Error())
		return err
	}
	return nil
}
