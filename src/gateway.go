package src

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	Resumeable               bool
	GatewayStatus

	// Default Gateway state.
	BotToken       string
	BotIntents     uint64
	BotVersion     string
	DiscordBaseURL string

	// Voice Gateway state.
}

func NewGateway() *Gateway {
	botToken := os.Getenv("DC_BOT_TOKEN")
	if len(botToken) == 0 {
		panic("provide dc_bot_token")
	}
	ver := os.Getenv("DC_VOICE_GATEWAY_VERSION")
	if len(ver) == 0 {
		panic("provide DC_VOICE_VERSION")
	}
	baseUrl := os.Getenv("DC_HTTP_BASE_URL")
	if len(baseUrl) == 0 {
		panic("provide DC_HTTP_BASE_URL")
	}
	u := url.URL{
		Scheme:   "wss",
		Host:     "gateway.discord.gg",
		RawQuery: fmt.Sprintf("v=%v&encoding=json", ver),
	}
	return &Gateway{
		HTTPClient:     http.DefaultClient,
		WSDialer:       websocket.DefaultDialer,
		WSUrl:          u.String(),
		BotToken:       botToken,
		BotIntents:     641,
		BotVersion:     ver,
		Resumeable:     false,
		GatewayStatus:  GatewayStatusDisconnected,
		DiscordBaseURL: baseUrl,
	}
}

func (g *Gateway) Open(ctx context.Context) error {
	log.Println("Attempting to connect to Discord.")
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
					switch d.Data.Name {
					case structs.CommandPlay:
						go g.handlePlayCmd(&d)
					default:
						interaction := new(structs.InteractionResponse)
						interaction.Type = structs.InteractionResponseTypeChannelMessageWithSource
						interaction.Data = structs.InteractionResponseDataMessage{
							Content: "hello",
						}
						go g.sendCallback(d.Token, d.ID, interaction)
					}
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
					g.GatewayStatus = GatewayStatusWaitingToIdentify
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
				log.Println("Heartbeat acknowledged.")
			case GatewayOpcodeReconnect:
				if g.Resumeable {
					g.reconnect()
				}
			}
		}
	}
}

func (g *Gateway) reconnect() {
	_ = g.Open(g.ctx)
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

func (g *Gateway) sendCallback(interactionToken string, interactionId string, data *structs.InteractionResponse) {
	rb, _ := json.Marshal(data)
	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	defer cancel()
	response, _ := g.sendHTTPRequest(ctx,
		http.MethodPost,
		fmt.Sprintf("https://discord.com/api/v%v/interactions/%s/%s/callback",
			g.BotVersion,
			interactionId,
			interactionToken),
		bytes.NewBuffer(rb))
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

func (g *Gateway) mentionUser(userId string) string {
	return fmt.Sprintf("<@%s>", userId)
}

func (g *Gateway) sendHTTPRequest(ctx context.Context, httpMethod string, url string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, httpMethod, url, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("Authorization", fmt.Sprintf("Bot %s", g.BotToken))
	request.Header.Set("User-Agent", "DiscordBot")
	response, err := g.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (g *Gateway) handlePlayCmd(interaction *structs.Interaction) {
	i := new(structs.InteractionResponse)
	i.Type = structs.InteractionResponseTypeChannelMessageWithSource

	response, err := g.sendHTTPRequest(g.ctx,
		http.MethodGet,
		fmt.Sprintf("%s/v%s/guilds/%s/voice-states/%s", g.DiscordBaseURL, g.BotVersion, interaction.GuildID, interaction.Member.User.ID),
		nil)
	if err != nil {
		i.Data = structs.InteractionResponseDataMessage{
			Content: "Failed to get current voice state.",
		}
		g.sendCallback(interaction.Token, interaction.ID, i)
		return
	}
	if response.StatusCode == http.StatusNotFound {
		i.Data = structs.InteractionResponseDataMessage{
			Content: fmt.Sprintf("%s, you are not connected to a voice channel. Please join a voice channel first to start using this feature.",
				g.mentionUser(interaction.Member.User.ID)),
		}
		g.sendCallback(interaction.Token, interaction.ID, i)
		return
	}
	defer response.Body.Close()
	b, _ := io.ReadAll(response.Body)
	voiceState := new(structs.VoiceState)
	_ = json.Unmarshal(b, voiceState)
	data := &GatewayVoiceState{
		GuildID:   interaction.GuildID,
		ChannelID: voiceState.ChannelID,
		SelfMute:  true,
		SelfDeaf:  false,
	}
	g.sendEvent(websocket.TextMessage, GatewayOpcodeVoiceStateUpdate, data)
}
