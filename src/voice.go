package src

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hendrywilliam/siren/src/structs"
)

type VoiceGatewayStatus = string

const (
	VoiceGatewayStatusConnected         VoiceGatewayStatus = "CONNECTED"
	VoiceGatewayStatusDisconnected      VoiceGatewayStatus = "DISCONNECTED"
	VoiceGatewayStatusWaitingToIdentify VoiceGatewayStatus = "WAITING_TO_IDENTIFY"
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

type Voice struct {
	rwlock     sync.RWMutex
	wsDialer   *websocket.Dialer
	wsConn     *websocket.Conn
	log        *slog.Logger
	ctx        context.Context
	cancelFunc context.CancelFunc

	voiceGatewayURL string
	sessionID       string
	serverID        string // Guild ID.
	userID          string
	token           string // Token for this current session.
	status          VoiceGatewayStatus
	mediaSessionID  string

	lastHeartbeatAcknowledge time.Time
	lastHeartbeatSent        time.Time
	heartbeatTicker          *time.Ticker

	botVersion string
	botToken   string // Token for HTTP request.

	udpConn        *net.UDPConn
	port           uint16
	ssrc           uint32
	ip             string
	encryptionMode string
	secretKeys     [32]byte
}

type NewVoiceArguments struct {
	SessionID         string
	UserID            string
	ServerID          string // Guild ID.
	DiscordBotVersion string
	DiscordBotToken   string
	Log               *slog.Logger
}

func NewVoice(args NewVoiceArguments) *Voice {
	voiceIdentifier := strings.Builder{}
	voiceIdentifier.WriteString("voice_")
	voiceIdentifier.WriteString(args.SessionID)
	return &Voice{
		wsDialer:                 websocket.DefaultDialer,
		lastHeartbeatAcknowledge: time.Now().Local(),
		botVersion:               args.DiscordBotVersion,
		botToken:                 args.DiscordBotToken,
		status:                   VoiceGatewayStatusDisconnected,
		log:                      args.Log.With(voiceIdentifier.String()),
	}
}

type VoiceOpenArgs struct {
	VoiceGatewayURL string
	VoiceToken      string
}

func (v *Voice) Open(ctx context.Context, args VoiceOpenArgs) {
	var err error
	cancelCtx, cancel := context.WithCancel(ctx)
	v.cancelFunc = cancel
	v.ctx = cancelCtx
	url := url.URL{
		Scheme:   "wss",
		Host:     args.VoiceGatewayURL,
		RawQuery: fmt.Sprintf("v=%s", v.botVersion),
	}
	v.wsConn, _, err = v.wsDialer.DialContext(v.ctx, url.String(), nil)
	if err != nil {
		// todo
		panic(err)
	}
	identifyEvent := structs.VoiceIdentify{
		ServerId:  v.serverID,
		UserID:    v.userID,
		SessionID: v.sessionID,
		Token:     args.VoiceToken,
	}
	err = v.sendEvent(websocket.TextMessage, VoiceOpcodeIdentify, identifyEvent)
	v.log.Info("Identify event sent.")
	if err == nil {
		go v.listen(v.wsConn)
	}
	return
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
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, VoiceCloseEventCodesDisconnected) {
					v.close()
					return
				}
				v.log.Error(err.Error())
				panic(err)
			}
			event, err := v.parseEvent(message)
			// v.log.Info(event)
			switch event.Op {
			case VoiceOpcodeHello:
				if d, ok := event.D.(structs.HelloEventData); ok {
					v.lastHeartbeatAcknowledge = v.getLocalTime()
					v.heartbeatTicker = time.NewTicker(time.Duration(d.HeartbeatInterval) * time.Millisecond)
					v.status = VoiceGatewayStatusWaitingToIdentify
					go v.heartbeating()
				}
			case VoiceOpcodeReady:
				if d, ok := event.D.(structs.VoiceReady); ok {
					v.ssrc = d.SSRC
					v.port = d.Port
					v.ip = d.IP
					v.encryptionMode = d.Modes[0]
					v.openUDPConn()
				}
			case VoiceOpcodeHeartbeatAck:
				v.lastHeartbeatAcknowledge = v.getLocalTime()
				v.log.Info("Heartbeat Acknowledged.")
			case VoiceOpcodeSessionDescription:
				if d, ok := event.D.(structs.SessionDescription); ok {
					v.secretKeys = d.SecretKey
				}
				v.sendSpeaking()
			case VoiceOpcodeClientsConnect:
				v.status = VoiceGatewayStatusConnected
				v.log.Info("Voice connected.")
			}
		}
	}
}

func (v *Voice) resume() error {
	voiceResumeData := &structs.VoiceResume{
		ServerID:  v.serverID,
		SessionID: v.sessionID,
		Token:     v.token,
	}
	err := v.sendEvent(websocket.TextMessage, VoiceOpcodeResume, voiceResumeData)
	if err != nil {
		v.log.Error("Failed to resume current session.")
		return err
	}
	return nil
}

func (v *Voice) close() {
	if v.heartbeatTicker != nil {
		v.heartbeatTicker.Stop()
		v.heartbeatTicker = nil
	}
	v.status = VoiceGatewayStatusDisconnected
	v.cancelFunc()
	v.wsConn.Close()
	v.log.Info("Connection closed.")
	return
}

func (v *Voice) heartbeating() {
	for {
		select {
		case <-v.ctx.Done():
			v.log.Info("Heartbeating stopped.")
			return
		case <-v.heartbeatTicker.C:
			hbData := structs.HeartbeatData{}
			hbData.T = v.getLastNonce()
			v.sendEvent(websocket.TextMessage, VoiceOpcodeHeartbeat, hbData)
			v.lastHeartbeatSent = v.getLocalTime()
			v.log.Info("Heartbeat event sent.")
		}
	}
}

func (v *Voice) getLocalTime() time.Time {
	return time.Now().UTC().Local()
}

func (v *Voice) sendEvent(messageType int, op structs.EventOpcode, d structs.GatewayEventData) error {
	data, err := json.Marshal(structs.GatewayEvent{
		Op: op,
		D:  d,
	})
	if err != nil {
		return fmt.Errorf("failed to marshall gateway event: %w", err)
	}
	v.rwlock.Lock()
	defer v.rwlock.Unlock()
	return v.wsConn.WriteMessage(messageType, data)
}

func (v *Voice) getLastNonce() int64 {
	return time.Now().UnixMilli()
}

func (v *Voice) parseError(err error) {
	// discord errors
	// if e, ok := err.(*websocket.CloseError); ok {
	// 	switch
	// }
	// // websocket internal errors
}

func (v *Voice) parseEvent(data []byte) (structs.GatewayEvent, error) {
	var event structs.GatewayEvent
	err := json.Unmarshal(data, &event)
	if err != nil {
		return structs.GatewayEvent{}, err
	}
	dataD, err := json.Marshal(event.D)
	switch event.Op {
	case VoiceOpcodeHello:
		var helloData structs.HelloEventData
		err = json.Unmarshal(dataD, &helloData)
		if err != nil {
			return structs.GatewayEvent{}, err
		}
		event.D = helloData
	case VoiceOpcodeReady:
		var readyData structs.VoiceReady
		err = json.Unmarshal(dataD, &readyData)
		if err != nil {
			return structs.GatewayEvent{}, err
		}
		event.D = readyData
	case VoiceOpcodeSessionDescription:
		var sessionDescriptionData structs.SessionDescription
		err = json.Unmarshal(dataD, &sessionDescriptionData)
		if err != nil {
			return structs.GatewayEvent{}, err
		}
		event.D = sessionDescriptionData
	case VoiceOpcodeClientsConnect:
		var clientsConnectData structs.VoiceClientsConnect
		err = json.Unmarshal(dataD, &clientsConnectData)
		if err != nil {
			return structs.GatewayEvent{}, err
		}
		event.D = clientsConnectData
	}
	return event, nil
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
	var buffData [100]byte
	_, err = v.udpConn.Read(buffData[:])
	if err != nil {
		return err
	}
	var ipAddrBuilder strings.Builder
	for i := 8; i < 25; i++ {
		if buffData[i] == 0 {
			break
		}
		ipAddrBuilder.Write(buffData[i : i+1])
	}
	ipAddr := ipAddrBuilder.String()
	port := binary.BigEndian.Uint16(buffData[72:74])
	return v.sendSelectProtocol(ipAddr, port)
}

func (v *Voice) sendSpeaking() error {
	speakingData := &structs.Speaking{
		Speaking: 1,
		Delay:    0,
		SSRC:     v.ssrc,
	}
	if err := v.sendEvent(websocket.TextMessage, VoiceOpcodeSpeaking, speakingData); err != nil {
		return err
	}
	v.log.Info("Speaking event sent.")
	return nil
}

func (v *Voice) sendSelectProtocol(ipAddr string, port uint16) error {
	eventData := &structs.SelectProtocol{
		Protocol: "udp",
		Data: structs.SelectProtocolData{
			Address: ipAddr,
			Port:    port,
			Mode:    "aead_aes256_gcm_rtpsize",
		},
	}
	err := v.sendEvent(websocket.TextMessage, VoiceOpcodeSelectProtocol, eventData)
	if err != nil {
		return err
	}
	v.log.Info("Select protocol event sent.")
	return nil
}

func (v *Voice) openUDPConn() error {
	var err error
	udpAdd, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%v", v.ip, v.port))
	if err != nil {
		return err
	}
	v.udpConn, err = net.DialUDP("udp", nil, udpAdd)
	if err != nil {
		return err
	}
	return v.sendIPDiscovery()
}
