package src

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	rwlock                   sync.RWMutex
	wsurl                    string
	resumeGatewayURL         string
	sessionID                string
	wsConn                   *websocket.Conn
	wsDialer                 *websocket.Dialer
	lastHeartbeatAcknowledge time.Time // local time.
	lastHeartbeatSent        time.Time // local time.
	sequence                 uint64
	httpClient               *http.Client
	ctx                      context.Context
	heartbeatTicker          *time.Ticker
	resumeable               bool
	status                   GatewayStatus

	botToken       string
	botIntents     uint64
	botVersion     string
	discordBaseURL string

	voiceManager *VoiceManager
	logger       *Logger
}

func NewGateway() *Gateway {
	botToken := os.Getenv("DC_BOT_TOKEN")
	if len(botToken) == 0 {
		panic("provide dc_bot_token")
	}
	ver := os.Getenv("DC_GATEWAY_VERSION")
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
		httpClient:     http.DefaultClient,
		wsDialer:       websocket.DefaultDialer,
		wsurl:          u.String(),
		botToken:       botToken,
		botIntents:     641,
		botVersion:     ver,
		resumeable:     false,
		status:         GatewayStatusDisconnected,
		discordBaseURL: baseUrl,
		voiceManager:   NewVoiceManager(),
		logger:         NewLogger("gateway"),
	}
}

func (g *Gateway) Open(ctx context.Context) error {
	g.logger.Info("connecting to discord...")
	g.ctx = ctx
	maxAttempts := 5
	err := g.retry(func() error {
		var err error
		g.wsConn, _, err = g.wsDialer.DialContext(g.ctx, g.wsurl, nil)
		if err != nil {
			return err
		}
		g.lastHeartbeatSent = g.getLocalTime()
		go g.listen(g.wsConn)
		return nil
	}, maxAttempts)
	if err != nil {
		g.logger.Fatal(err)
	}
	return nil
}

func (g *Gateway) retry(fn func() error, maxAttempts int) error {
	for attempts := 1; attempts <= maxAttempts; attempts++ {
		err := fn()
		if err == nil {
			return nil
		}
		g.logger.Error(fmt.Errorf("connection failed. reconnecting..."))
		delay := time.Duration(math.Pow(2, float64((attempts-1)))*1000) * time.Millisecond
		select {
		case <-time.After(delay):
			continue
		case <-g.ctx.Done():
			return nil
		}
	}
	return fmt.Errorf("failed to open a connection after several attempts")
}

func (g *Gateway) listen(conn *websocket.Conn) {
	for {
		select {
		case <-g.ctx.Done():
			g.logger.Info("stop listening.")
			return
		default:
			g.rwlock.Lock()
			same := g.wsConn == conn
			g.rwlock.Unlock()
			if !same {
				// If the connection is not the same
				// it means that we have opened a new connection
				// we simply exit last "listen" goroutine.
				return
			}
			_, message, err := conn.ReadMessage()
			if err != nil {
				panic(err)
			}
			event, err := g.parseEvent(message)
			g.sequence = event.S
			// g.logger.JSON(event)
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
					g.resumeGatewayURL = d.ResumeGatewayURL
					g.sessionID = d.SessionID
					g.status = GatewayStatusReady
					g.resumeable = true
					g.logger.SetAttr("id", d.SessionID)
					g.logger.Info("connection established")
				case VoiceStateUpdateData:
					if voice := g.voiceManager.Get(d.GuildID); voice != nil {
						if len(d.ChannelID) == 0 {
							g.voiceManager.Delete(d.GuildID)
						}
						continue
					}
					voice := NewVoice()
					voice.sessionID = d.SessionID
					voice.userID = d.UserID
					voice.serverID = d.GuildID
					g.voiceManager.Add(d.GuildID, voice)
				case VoiceServerUpdateData:
					voice := g.voiceManager.Get(d.GuildID)
					if voice != nil {
						voice.voiceGatewayURL = d.Endpoint
						voice.token = d.Token
						voice.Open(g.ctx)
					}
				}
			case GatewayOpcodeHello:
				if d, ok := event.D.(HelloEventData); ok {
					g.lastHeartbeatAcknowledge = g.getLocalTime()
					g.heartbeatTicker = time.NewTicker(time.Duration(d.HeartbeatInterval) * time.Millisecond)
					g.status = GatewayStatusWaitingToIdentify
					go g.heartbeating()
					if g.status == GatewayStatusWaitingToIdentify {
						identifyEv := IdentifyEventData{
							Token:   g.botToken,
							Intents: g.botIntents,
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
			case GatewayOpcodeHeartbeat:
				g.sendEvent(websocket.TextMessage, GatewayOpcodeHeartbeat, g.getLastNonce())
			case GatewayOpcodeHeartbeatAck:
				g.lastHeartbeatAcknowledge = g.getLocalTime()
				g.logger.Info("Gateway heartbeat acknowledged.")
			case GatewayOpcodeReconnect:
				if g.resumeable {
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
	defer g.logger.Info("Gateway heartbeating stopped.")
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-g.heartbeatTicker.C:
			g.sendEvent(websocket.TextMessage, GatewayOpcodeHeartbeat, g.getLastNonce())
			g.lastHeartbeatSent = g.getLocalTime()
			g.logger.Info("Gateway heartbeat event sent.")
		}
	}
}

func (g *Gateway) close() {
	if g.heartbeatTicker != nil {
		g.heartbeatTicker.Stop()
		g.logger.Info("Gateway heartbeat ticker stopped.")
	}
	g.wsConn.Close()
	g.logger.Info("Gateway connection stopped.")
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
	g.rwlock.Lock()
	defer g.rwlock.Unlock()
	return g.wsConn.WriteMessage(messageType, data)
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
		case structs.EventNameVoiceStateUpdate:
			var data VoiceStateUpdateData
			err = json.Unmarshal(dataD, &data)
			if err != nil {
				return GatewayEvent{}, err
			}
			event.D = data
		case structs.EventNameVoiceServerUpdate:
			var data VoiceServerUpdateData
			err = json.Unmarshal(dataD, &data)
			if err != nil {
				return GatewayEvent{}, err
			}
			event.D = data
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
			g.botVersion,
			interactionId,
			interactionToken),
		bytes.NewBuffer(rb))
	if response.StatusCode == http.StatusNoContent {
		g.logger.Info("interaction callback sent.")
		return
	}
	g.logger.Warn("failed to send callback interaction.")
	return
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
	request.Header.Set("Authorization", fmt.Sprintf("Bot %s", g.botToken))
	request.Header.Set("User-Agent", "DiscordBot")
	response, err := g.httpClient.Do(request)
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
		fmt.Sprintf("%s/v%s/guilds/%s/voice-states/%s", g.discordBaseURL, g.botVersion, interaction.GuildID, interaction.Member.User.ID),
		nil)
	if err != nil {
		i.Data.Content = "failed to get current voice state."
		g.sendCallback(interaction.Token, interaction.ID, i)
		return
	}
	if response.StatusCode == http.StatusNotFound {
		i.Data.Content = fmt.Sprintf("%s, please join a voice channel first to start using this feature.",
			g.mentionUser(interaction.Member.User.ID))
		g.sendCallback(interaction.Token, interaction.ID, i)
		return
	}
	defer response.Body.Close()
	b, _ := io.ReadAll(response.Body)
	voiceState := new(VoiceState)
	json.Unmarshal(b, voiceState)
	data := &GatewayVoiceState{
		GuildID:   interaction.GuildID,
		ChannelID: voiceState.ChannelID,
		SelfMute:  false,
		SelfDeaf:  false,
	}
	g.sendEvent(websocket.TextMessage, GatewayOpcodeVoiceStateUpdate, data)
}
