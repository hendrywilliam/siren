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
	heartbeatDuration        time.Duration

	BotToken   string
	BotIntents uint64
	BotVersion int
	GatewayStatus
}

func NewGateway(ctx context.Context) *Gateway {
	const DISCORD_API_VERSION = 10
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
		HTTPClient:        http.DefaultClient,
		WSDialer:          websocket.DefaultDialer,
		ctx:               ctx,
		heartbeatDuration: 2 * time.Second,
		WSUrl:             u.String(),
		BotToken:          botToken,
		BotIntents:        641,
		BotVersion:        DISCORD_API_VERSION,
		GatewayStatus:     GatewayStatusWaitingToIdentify,
	}
}

func (g *Gateway) Open() error {
	var err error
	log.Println("Attempting to connect to discord")
	g.WSConn, _, err = g.WSDialer.DialContext(g.ctx, g.WSUrl, nil)
	if err != nil {
		g.WSConn.Close()
		return fmt.Errorf("Failed to connect to discord gateway: %w", err)
	}
	g.LastHeartbeatSent = g.getLocalTime()
	go g.listen()
	return nil
}

// listen to inbound event.
func (g *Gateway) listen() {
	defer g.WSConn.Close()
	for {
		select {
		case <-g.ctx.Done():
			log.Println("stop listening")
			return
		default:
			_, message, err := g.WSConn.ReadMessage()
			if err != nil {
				panic(err)
			}
			event, err := g.parseEvent(message)
			switch d := event.D.(type) {
			case HelloEventData:
				g.LastHeartbeatAcknowledge = g.getLocalTime()
				g.heartbeatTicker = time.NewTicker(time.Duration(d.HeartbeatInterval) * time.Millisecond)
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
						panic("Failed to identify current session.")
					}
				}
			case structs.Interaction:
				interaction := new(structs.InteractionResponse)
				interaction.Type = structs.InteractionResponseTypeChannelMessageWithSource
				interaction.Data = structs.InteractionResponseDataMessage{
					Content: "hello",
				}
				go g.sendCallback(d.Token, d.ID, interaction)
			default:
				continue
			}
		}
	}
}

func (g *Gateway) reconnect() {

}

func (g *Gateway) heartbeating() {
	defer g.heartbeatTicker.Stop()
	defer log.Println("Heartbeating stopped.")
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-g.heartbeatTicker.C:
			lastNonce := time.Now().UnixMilli()
			_ = g.sendEvent(websocket.TextMessage, GatewayOpcodeHeartbeat, lastNonce)
			log.Println("Heartbeating sent.")
		}
	}
}

func (g *Gateway) close() {}

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
		var dispatchData structs.Interaction
		err = json.Unmarshal(dataD, &dispatchData)
		if err != nil {
			return GatewayEvent{}, err
		}
		event.D = dispatchData
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
	}
	return
}
