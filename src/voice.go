package src

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type VoiceGatewayStatus = string

const (
	VoiceGatewayStatusConnected         VoiceGatewayStatus = "CONNECTED"
	VoiceGatewayStatusDisconnected      VoiceGatewayStatus = "DISCONNECTED"
	VoiceGatewayStatusWaitingToIdentify VoiceGatewayStatus = "WAITING_TO_IDENTIFY"
)

// voice gateway
type Voice struct {
	rwlock     sync.RWMutex
	wsDialer   *websocket.Dialer
	wsConn     *websocket.Conn
	logger     *Logger
	ctx        context.Context
	cancelFunc context.CancelFunc

	voiceGatewayURL string
	sessionID       string
	serverID        string // guild id.
	userID          string
	token           string // token for this current session.
	status          GatewayStatus
	mediaSessionID  string

	lastHeartbeatAcknowledge time.Time
	lastHeartbeatSent        time.Time
	heartbeatTicker          *time.Ticker

	botVersion string
	botToken   string // for http request.

	udpConn        *net.UDPConn
	port           uint16
	ssrc           uint32
	ip             string
	encryptionMode string
	secretKeys     [32]byte
}

func NewVoice() *Voice {
	botVersion := os.Getenv("DC_VOICE_GATEWAY_VERSION")
	if len(botVersion) == 0 {
		panic("provide DC_VOICE_GATEWAY_VERSION")
	}
	botToken := os.Getenv("DC_BOT_TOKEN")
	if len(botToken) == 0 {
		panic("provide dc_bot_token")
	}
	return &Voice{
		wsDialer:                 websocket.DefaultDialer,
		lastHeartbeatAcknowledge: time.Now().Local(),
		botVersion:               botVersion,
		botToken:                 botToken,
		status:                   GatewayStatusDisconnected,
		logger:                   NewLogger(),
	}
}

func (v *Voice) Open(ctx context.Context) {
	var err error
	cancelCtx, cancel := context.WithCancel(ctx)
	v.cancelFunc = cancel
	v.ctx = cancelCtx
	url := url.URL{
		Scheme:   "wss",
		Host:     v.voiceGatewayURL,
		RawQuery: fmt.Sprintf("v=%s", v.botVersion),
	}
	v.wsConn, _, err = v.wsDialer.DialContext(v.ctx, url.String(), nil)
	if err != nil {
		panic(err)
	}
	identifyEvent := VoiceIdentify{
		ServerId:  v.serverID,
		UserID:    v.userID,
		SessionID: v.sessionID,
		Token:     v.token,
	}
	v.sendEvent(websocket.TextMessage, VoiceOpcodeIdentify, identifyEvent)
	v.logger.Info("Identify event sent.", "ID", v.sessionID)
	go v.listen(v.wsConn)
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
				panic(err)
			}
			event, err := v.parseEvent(message)
			v.logger.JSON(event)
			switch event.Op {
			case VoiceOpcodeHello:
				if d, ok := event.D.(HelloEventData); ok {
					v.lastHeartbeatAcknowledge = v.getLocalTime()
					v.heartbeatTicker = time.NewTicker(time.Duration(d.HeartbeatInterval) * time.Millisecond)
					v.status = GatewayStatusWaitingToIdentify
					go v.heartbeating()
				}
			case VoiceOpcodeReady:
				if d, ok := event.D.(VoiceReady); ok {
					v.ssrc = d.SSRC
					v.port = d.Port
					v.ip = d.IP
					v.encryptionMode = d.Modes[0]
					_ = v.OpenUDPConn()
				}
			case VoiceOpcodeHeartbeatAck:
				v.lastHeartbeatAcknowledge = v.getLocalTime()
				v.logger.Info("Voice Heartbeat acknowledged.", "ID", v.sessionID)
			case VoiceOpcodeSessionDescription:
				if d, ok := event.D.(SessionDescription); ok {
					v.secretKeys = d.SecretKey
				}
				v.sendSpeaking()
			case VoiceOpcodeClientsConnect:
				v.status = VoiceGatewayStatusConnected
				v.logger.Info("Voice connected.", "ID", v.sessionID)
			}
		}
	}
}

func (v *Voice) close() {
	defer v.logger.Info("Voice connection closed.", "ID", v.sessionID)
	if v.heartbeatTicker != nil {
		v.heartbeatTicker.Stop()
		v.heartbeatTicker = nil
	}
	v.status = VoiceGatewayStatusDisconnected
	v.cancelFunc()
	v.wsConn.Close()
	return
}

func (v *Voice) heartbeating() {
	heartbeatData := &HeartbeatData{}
	defer v.logger.Info("Voice heartbeating stopped.", "ID", v.sessionID)
	for {
		select {
		case <-v.ctx.Done():
			return
		case <-v.heartbeatTicker.C:
			heartbeatData.T = v.getLastNonce()
			_ = v.sendEvent(websocket.TextMessage, VoiceOpcodeHeartbeat, heartbeatData)
			v.lastHeartbeatSent = v.getLocalTime()
			v.logger.Info("Voice Heartbeat event sent.", "ID", v.sessionID)
		}
	}
}

func (v *Voice) getLocalTime() time.Time {
	return time.Now().UTC().Local()
}

func (v *Voice) sendEvent(messageType int, op EventOpcode, d GatewayEventData) error {
	data, err := json.Marshal(GatewayEvent{
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

func (v *Voice) parseEvent(data []byte) (GatewayEvent, error) {
	var event GatewayEvent
	err := json.Unmarshal(data, &event)
	if err != nil {
		return GatewayEvent{}, err
	}
	dataD, err := json.Marshal(event.D)
	switch event.Op {
	case VoiceOpcodeHello:
		var helloData HelloEventData
		err = json.Unmarshal(dataD, &helloData)
		if err != nil {
			return GatewayEvent{}, err
		}
		event.D = helloData
	case VoiceOpcodeReady:
		var readyData VoiceReady
		err = json.Unmarshal(dataD, &readyData)
		if err != nil {
			return GatewayEvent{}, err
		}
		event.D = readyData
	case VoiceOpcodeSessionDescription:
		var sessionDescriptionData SessionDescription
		err = json.Unmarshal(dataD, &sessionDescriptionData)
		if err != nil {
			return GatewayEvent{}, err
		}
		event.D = sessionDescriptionData
	case VoiceOpcodeClientsConnect:
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
	defer v.logger.Info("Speaking event sent.")
	speakingData := Speaking{
		Speaking: 1,
		Delay:    0,
		SSRC:     v.ssrc,
	}
	return v.sendEvent(websocket.TextMessage, VoiceOpcodeSpeaking, &speakingData)
}

func (v *Voice) sendSelectProtocol(ipAddr string, port uint16) error {
	eventData := &SelectProtocol{
		Protocol: "udp",
		Data: SelectProtocolData{
			Address: ipAddr,
			Port:    port,
			Mode:    "aead_aes256_gcm_rtpsize",
		},
	}
	err := v.sendEvent(websocket.TextMessage, VoiceOpcodeSelectProtocol, eventData)
	if err != nil {
		return err
	}
	v.logger.Info("Select Protocol event sent.", "ID", v.sessionID)
	return nil
}

func (v *Voice) OpenUDPConn() error {
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
