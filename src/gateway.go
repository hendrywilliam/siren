package src

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hendrywilliam/siren/src/structs"
)

type GatewayStatus = string

const (
	GatewayStatusWaitingToIdentify GatewayStatus = "WAITING_TO_IDENTIFY"
	GatewayStatusReady             GatewayStatus = "READY"
	GatewayStatusDisconnected      GatewayStatus = "DISCONNECTED"
)

type Gateway struct {
	RWLock                   sync.RWMutex
	WSUrl                    string
	ResumeGatewayURL         string
	SessionID                string
	WSConn                   *websocket.Conn
	WSDialer                 *websocket.Dialer
	LastHeartbeatAcknowledge time.Time // local time.
	LastHeartbeatSent        time.Time // local time.
	Sequence                 uint64
	HTTPClient               *http.Client
	ctx                      context.Context
	heartbeatTicker          *time.Ticker
	BotToken                 string
	BotIntents               uint64
	BotVersion               int
	Resumeable               bool
	GatewayStatus
}

const DISCORD_API_VERSION = 10
const BOT_INTENTS = 641

func NewGateway() *Gateway {
	botToken := os.Getenv("DC_BOT_TOKEN")
	if len(botToken) == 0 {
		panic("provide dc_bot_token")
	}
	u := url.URL{
		Scheme:   "wss",
		Host:     "gateway.discord.gg",
		RawQuery: fmt.Sprintf("v=%v&encoding=json", DISCORD_API_VERSION),
	}
	return &Gateway{
		HTTPClient:    http.DefaultClient,
		WSDialer:      websocket.DefaultDialer,
		WSUrl:         u.String(),
		BotToken:      botToken,
		BotIntents:    641,
		BotVersion:    DISCORD_API_VERSION,
		Resumeable:    false,
		GatewayStatus: GatewayStatusWaitingToIdentify,
	}
}

func (g *Gateway) Open(ctx context.Context) error {
	log.Println("Attempting to connect to discord.")
	var err error
	g.ctx = ctx
	g.WSConn, _, err = g.WSDialer.DialContext(g.ctx, g.WSUrl, nil)
	if err != nil {
		g.WSConn.Close()
		return fmt.Errorf("Failed to connect to discord gateway: %w", err)
	}
	g.LastHeartbeatSent = g.getLocalTime()
	go g.listen(g.WSConn)
	return nil
}

// listen to inbound event.
func (g *Gateway) listen(conn *websocket.Conn) {
	for {
		select {
		case <-g.ctx.Done():
			log.Println("stop listening")
			return
		default:
			g.RWLock.Lock()
			same := g.WSConn == conn
			g.RWLock.Unlock()
			if !same {
				// If the connection is not the same
				// it means that we have opened a new connection
				// we simply exit last "listen" goroutine.
				return
			}
			_, message, err := conn.ReadMessage()
			if err != nil {
				// should change to reconnect.
			}
			event, err := g.parseEvent(message)
			g.Sequence = event.S
			g.printPrettyJson(event)
			switch event.Op {
			case GatewayOpcodeDispatch:
				switch d := event.D.(type) {
				case structs.Interaction:
					interaction := new(structs.InteractionResponse)
					interaction.Type = structs.InteractionResponseTypeChannelMessageWithSource
					interaction.Data = structs.InteractionResponseDataMessage{
						Content: "hello",
					}
					go g.sendCallback(d.Token, d.ID, interaction)
				case structs.ReadyEventData:
					g.ResumeGatewayURL = d.ResumeGatewayURL
					g.SessionID = d.SessionID
					g.GatewayStatus = GatewayStatusReady
					g.Resumeable = true
					log.Println("Connection established.")
				}
			case GatewayOpcodeHello:
				if d, ok := event.D.(HelloEventData); ok {
					g.LastHeartbeatAcknowledge = g.getLocalTime()
					g.heartbeatTicker = time.NewTicker(time.Duration(d.HeartbeatInterval) * time.Millisecond)
					// dc may send heartbeat, we need to send heartbeat back immediately.
					if g.GatewayStatus == GatewayStatusReady {
						_ = g.sendEvent(websocket.TextMessage, GatewayOpcodeHeartbeat, g.getLastNonce())
						continue
					}
					go g.heartbeating()
					if g.GatewayStatus == GatewayStatusWaitingToIdentify {
						identifyEv := IdentifyEventData{
							Token:   g.BotToken,
							Intents: g.BotIntents,
							Properties: IdentifyEventDProperties{
								Os:      "ubuntu",
								Browser: "chrome",
								Device:  "refrigerator",
							},
						}
						err := g.sendEvent(websocket.TextMessage, GatewayOpcodeIdentify, identifyEv)
						if err != nil {
							panic("failed to identify current session.")
						}
					}
				}
			case GatewayOpcodeHeartbeatAck:
				g.LastHeartbeatAcknowledge = g.getLocalTime()
			case GatewayOpcodeReconnect:
				if g.Resumeable {
					g.reconnect()
				}
			}
		}
	}
}

func (g *Gateway) reconnect() {
	g.Open(g.ctx)
	return
}

func (g *Gateway) heartbeating() {
	defer g.heartbeatTicker.Stop()
	defer log.Println("Heartbeating stopped.")
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-g.heartbeatTicker.C:
			_ = g.sendEvent(websocket.TextMessage, GatewayOpcodeHeartbeat, g.getLastNonce())
			g.LastHeartbeatSent = g.getLocalTime()
			log.Println("Heartbeat event sent.")
		}
	}
}

func (g *Gateway) close() {
	if g.heartbeatTicker != nil {
		g.heartbeatTicker.Stop()
		log.Println("Heartbeat Ticker stopped.")
	}
	g.WSConn.Close()
	log.Println("Connection stopped.")
	return
}

func (g *Gateway) sendEvent(messageType int, op EventOpcode, d GatewayEventData) error {
	data, err := json.Marshal(GatewayEvent{
		Op: op,
		D:  d,
	})
	if err != nil {
		return fmt.Errorf("failed to marshall gateway event: %w", err)
	}
	g.RWLock.Lock()
	defer g.RWLock.Unlock()
	return g.WSConn.WriteMessage(messageType, data)
}

func (g *Gateway) getLocalTime() time.Time {
	return time.Now().UTC().Local()
}

func (g *Gateway) getLastNonce() int64 {
	return time.Now().UnixMilli()
}

func (g *Gateway) parseEvent(data []byte) (GatewayEvent, error) {
	var event GatewayEvent
	err := json.Unmarshal(data, &event)
	if err != nil {
		return GatewayEvent{}, err
	}
	dataD, err := json.Marshal(event.D)
	switch event.Op {
	case GatewayOpcodeHello:
		var helloData HelloEventData
		err = json.Unmarshal(dataD, &helloData)
		if err != nil {
			return GatewayEvent{}, err
		}
		event.D = helloData
	case GatewayOpcodeDispatch:
		switch event.T {
		case structs.EventNameReady:
			var dispatchData structs.ReadyEventData
			err = json.Unmarshal(dataD, &dispatchData)
			if err != nil {
				return GatewayEvent{}, err
			}
			event.D = dispatchData
		case structs.EventNameInteractionCreate:
			var dispatchData structs.Interaction
			err = json.Unmarshal(dataD, &dispatchData)
			if err != nil {
				return GatewayEvent{}, err
			}
			event.D = dispatchData
		}
	}
	return event, nil
}

// send interaction callback over http.
func (g *Gateway) sendCallback(interactionToken string, interactionId string, data *structs.InteractionResponse) {
	rb, _ := json.Marshal(data)
	request, _ := http.NewRequestWithContext(
		g.ctx,
		http.MethodPost,
		fmt.Sprintf("https://discord.com/api/v%v/interactions/%s/%s/callback",
			g.BotVersion,
			interactionId,
			interactionToken),
		bytes.NewBuffer(rb),
	)
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, _ := g.HTTPClient.Do(request)
	if response.StatusCode == http.StatusNoContent {
		log.Println("Interaction callback sent.")
	} else {
		log.Println("Failed to send callback interaction.")
	}
	return
}

func (g *Gateway) printPrettyJson(data any) {
	prettyJson, err := json.MarshalIndent(data, "  ", "  ")
	if err != nil {
		return
	}
	log.Printf("\n%s\n", string(prettyJson))
}
