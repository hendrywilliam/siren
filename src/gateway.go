package src

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hendrywilliam/siren/src/structs"
	"github.com/hendrywilliam/siren/src/utils"
)

type GatewayStatus = string

const (
	GatewayStatusWaitingToIdentify GatewayStatus = "WAITING_TO_IDENTIFY"
	GatewayStatusReady             GatewayStatus = "READY"
	GatewayStatusDisconnected      GatewayStatus = "DISCONNECTED"
)

type GatewayOpcode = uint8

const (
	GatewayOpcodeDispatch                GatewayOpcode = 0
	GatewayOpcodeHeartbeat               GatewayOpcode = 1
	GatewayOpcodeIdentify                GatewayOpcode = 2
	GatewayOpcodePresenceUpdate          GatewayOpcode = 3
	GatewayOpcodeVoiceStateUpdate        GatewayOpcode = 4
	GatewayOpcodeResume                  GatewayOpcode = 6
	GatewayOpcodeReconnect               GatewayOpcode = 7
	GatewayOpcodeRequestGuildMember      GatewayOpcode = 8
	GatewayOpcodeInvalidSession          GatewayOpcode = 9
	GatewayOpcodeHello                   GatewayOpcode = 10
	GatewayOpcodeHeartbeatAck            GatewayOpcode = 11
	GatewayOpcodeRequestSoundboardSounds GatewayOpcode = 31
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

	voiceManager VoiceManager
	logger       *slog.Logger
}

type GatewayArguments struct {
	Config                utils.AppConfig
	Logger                *slog.Logger
	DiscordBotToken       string
	DiscordGatewayVersion string
	DiscordHTTPBaseURL    string
}

func NewGateway(cfg GatewayArguments) *Gateway {
	u := url.URL{
		Scheme:   "wss",
		Host:     "gateway.discord.gg",
		RawQuery: fmt.Sprintf("v=%v&encoding=json", cfg.DiscordGatewayVersion),
	}
	return &Gateway{
		httpClient:     http.DefaultClient,
		wsDialer:       websocket.DefaultDialer,
		wsurl:          u.String(),
		botToken:       cfg.DiscordBotToken,
		botIntents:     641,
		botVersion:     cfg.DiscordGatewayVersion,
		resumeable:     false,
		status:         GatewayStatusDisconnected,
		discordBaseURL: cfg.DiscordHTTPBaseURL,
		voiceManager:   NewVoiceManager(),
		logger:         cfg.Logger,
	}
}

func (g *Gateway) Open(ctx context.Context) error {
	g.logger.Info("Connecting to discord...")
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
		g.logger.Error(err.Error())
		os.Exit(1)
	}
	return nil
}

func (g *Gateway) retry(fn func() error, maxAttempts int) error {
	for attempts := 1; attempts <= maxAttempts; attempts++ {
		err := fn()
		if err == nil {
			return nil
		}
		g.logger.Error("Connection failed. Reconnecting...")
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
					g.logger.Info("Connection established. Gateway is ready.")
				case structs.VoiceStateUpdateData:
					g.HandleVoiceStateUpdate(d)
				case structs.VoiceServerUpdateData:
					g.HandleVoiceServerUpdate(d)
				}
			case GatewayOpcodeHello:
				if d, ok := event.D.(structs.HelloEventData); ok {
					g.lastHeartbeatAcknowledge = g.getLocalTime()
					g.heartbeatTicker = time.NewTicker(time.Duration(d.HeartbeatInterval) * time.Millisecond)
					g.status = GatewayStatusWaitingToIdentify
					go g.heartbeating()
					if g.status == GatewayStatusWaitingToIdentify {
						identifyEv := structs.IdentifyEventData{
							Token:   g.botToken,
							Intents: g.botIntents,
							Properties: structs.IdentifyEventDProperties{
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

func (g *Gateway) sendEvent(messageType int, op structs.EventOpcode, d structs.GatewayEventData) error {
	data, err := json.Marshal(structs.GatewayEvent{
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

func (g *Gateway) parseEvent(data []byte) (structs.GatewayEvent, error) {
	var event structs.GatewayEvent
	err := json.Unmarshal(data, &event)
	if err != nil {
		return structs.GatewayEvent{}, err
	}
	dataD, err := json.Marshal(event.D)
	switch event.Op {
	case GatewayOpcodeHello:
		var helloData structs.HelloEventData
		err = json.Unmarshal(dataD, &helloData)
		if err != nil {
			return structs.GatewayEvent{}, err
		}
		event.D = helloData
	case GatewayOpcodeDispatch:
		switch event.T {
		case structs.EventNameReady:
			var dispatchData structs.ReadyEventData
			err = json.Unmarshal(dataD, &dispatchData)
			if err != nil {
				return structs.GatewayEvent{}, err
			}
			event.D = dispatchData
		case structs.EventNameInteractionCreate:
			var dispatchData structs.Interaction
			err = json.Unmarshal(dataD, &dispatchData)
			if err != nil {
				return structs.GatewayEvent{}, err
			}
			event.D = dispatchData
		case structs.EventNameVoiceStateUpdate:
			var data structs.VoiceStateUpdateData
			err = json.Unmarshal(dataD, &data)
			if err != nil {
				return structs.GatewayEvent{}, err
			}
			event.D = data
		case structs.EventNameVoiceServerUpdate:
			var data structs.VoiceServerUpdateData
			err = json.Unmarshal(dataD, &data)
			if err != nil {
				return structs.GatewayEvent{}, err
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
		fmt.Sprintf("%s/v%s/guilds/%s/voice-states/%s",
			g.discordBaseURL,
			g.botVersion,
			interaction.GuildID,
			interaction.Member.User.ID),
		nil)
	if err != nil {
		i.Data.Content = "failed to get current voice state."
		g.sendCallback(interaction.Token, interaction.ID, i)
		return
	}
	if response.StatusCode == http.StatusNotFound {
		i.Data.Content = fmt.Sprintf("%s, Please join to a voice channel before using this command.",
			g.mentionUser(interaction.Member.User.ID))
		g.sendCallback(interaction.Token, interaction.ID, i)
		return
	}
	defer response.Body.Close()
	b, _ := io.ReadAll(response.Body)
	voiceState := new(structs.VoiceState)
	json.Unmarshal(b, voiceState)
	data := &structs.GatewayVoiceState{
		GuildID:   interaction.GuildID,
		ChannelID: voiceState.ChannelID,
		SelfMute:  false,
		SelfDeaf:  false,
	}
	g.sendEvent(websocket.TextMessage, GatewayOpcodeVoiceStateUpdate, data)
}

func (g *Gateway) HandleVoiceStateUpdate(data structs.VoiceStateUpdateData) {
	if voice := g.voiceManager.Get(data.GuildID); voice != nil {
		if len(data.ChannelID) == 0 {
			g.voiceManager.Delete(data.GuildID)
		}
		return
	}
	nv := NewVoice(NewVoiceArguments{
		SessionID: data.SessionID,
		UserID:    data.UserID,
		ServerID:  data.GuildID, // Server ID = Guild ID.
	})
	g.voiceManager.Add(data.GuildID, nv)
	return
}

func (g *Gateway) HandleVoiceServerUpdate(data structs.VoiceServerUpdateData) {
	if v := g.voiceManager.Get(data.GuildID); v != nil {
		v.Open(g.ctx, VoiceOpenArgs{
			VoiceGatewayURL: data.Endpoint,
			VoiceToken:      data.Token,
		})
	}
	return
}
