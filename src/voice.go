package src

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// voice gateway
type Voice struct {
	rwlock   sync.RWMutex
	wsDialer *websocket.Dialer
	wsConn   *websocket.Conn

	// one guild state
	voiceGatewayURL string
	sessionID       string
	serverID        string
	userID          string
	status          GatewayStatus

	lastHeartbeatAcknowledge time.Time
	lastHeartbeatSent        time.Time
	heartbeatTicker          *time.Ticker

	ctx   context.Context
	token string

	ssrc       uint16
	botVersion string
	botToken   string // for http request
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
	}
}

func (v *Voice) Open(ctx context.Context) {
	v.ctx = ctx
	var err error
	url := url.URL{
		Scheme:   "wss",
		Host:     v.voiceGatewayURL,
		RawQuery: fmt.Sprintf("v=%s", v.botVersion),
	}
	v.wsConn, _, err = v.wsDialer.DialContext(ctx, url.String(), nil)
	if err != nil {
		// should handle error instead of panic
		// do it later.
		panic(err)
	}
	go v.listen(v.wsConn)
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
			log.Println(string(message))
			if err != nil {
				// should handle error instead of panic
				// do it later.
				panic(err)
			}
			event, err := v.parseEvent(message)
			v.printPrettyJson(event)
			switch event.Op {
			case VoiceOpcodeHello:
				if d, ok := event.D.(HelloEventData); ok {
					v.lastHeartbeatAcknowledge = v.getLocalTime()
					v.heartbeatTicker = time.NewTicker(time.Duration(d.HeartbeatInterval) * time.Millisecond)
					v.status = GatewayStatusWaitingToIdentify
					go v.heartbeating()

					if v.status == GatewayStatusWaitingToIdentify {
						identifyEvent := VoiceIdentify{
							ServerId:  v.serverID,
							UserID:    v.userID,
							SessionID: v.sessionID,
							Token:     v.token,
						}
						v.sendEvent(websocket.TextMessage, VoiceOpcodeIdentify, identifyEvent)
					}
				}
			case VoiceOpcodeReady:
				if d, ok := event.D.(VoiceReady); ok {
					log.Println(d)
				}
			}
		}
	}
}

func (v *Voice) heartbeating() {
	defer v.heartbeatTicker.Stop()
	defer log.Println("Heartbeating stopped.")
	for {
		select {
		case <-v.ctx.Done():
			return
		case <-v.heartbeatTicker.C:
			_ = v.sendEvent(websocket.TextMessage, GatewayOpcodeHeartbeat, v.getLastNonce())
			v.lastHeartbeatSent = v.getLocalTime()
			log.Println("Heartbeat event sent.")
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
	}
	return event, nil
}

func (v *Voice) printPrettyJson(data any) {
	prettyJson, err := json.MarshalIndent(data, "  ", "  ")
	if err != nil {
		return
	}
	log.Printf("\n%s\n", string(prettyJson))
}
