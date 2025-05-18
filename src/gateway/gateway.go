package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hendrywilliam/siren/src/structs"
	"github.com/hendrywilliam/siren/src/voice"
	"github.com/hendrywilliam/siren/src/voicemanager"
)

type GatewayStatus = string

const (
	StatusReady        GatewayStatus = "READY"
	StatusDisconnected GatewayStatus = "DISCONNECTED"
)

type GatewayOpcode = int

const (
	OpcodeDispatch                GatewayOpcode = 0
	OpcodeHeartbeat               GatewayOpcode = 1
	OpcodeIdentify                GatewayOpcode = 2
	OpcodePresenceUpdate          GatewayOpcode = 3
	OpcodeVoiceStateUpdate        GatewayOpcode = 4
	OpcodeResume                  GatewayOpcode = 6
	OpcodeReconnect               GatewayOpcode = 7
	OpcodeRequestGuildMember      GatewayOpcode = 8
	OpcodeInvalidSession          GatewayOpcode = 9
	OpcodeHello                   GatewayOpcode = 10
	OpcodeHeartbeatAck            GatewayOpcode = 11
	OpcodeRequestSoundboardSounds GatewayOpcode = 31
)

type GatewayCloseEventCode = int

const (
	UnknownError GatewayCloseEventCode = iota + 4000
	UnknownOpcode
	DecodeError
	NotAuthenticated
	AuthenticationFailed
	AlreadyAuthenticated
)

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrNotAuthenticated     = errors.New("not authenticated")
	ErrDecode               = errors.New("invalid payload")
	ErrGatewayIsAlreadyOpen = errors.New("gateway is already open")
	ErrUnknown              = errors.New("unknown error")
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
	sequence                 atomic.Uint64
	httpClient               *http.Client
	ctx                      context.Context
	heartbeatTicker          *time.Ticker
	resumeable               bool
	status                   GatewayStatus

	botToken       string
	botIntents     uint
	botVersion     uint
	discordBaseURL string

	voiceManager voicemanager.VoiceManager
	log          *slog.Logger
}

type DiscordArguments struct {
	BotToken  string
	BotIntent uint

	Logger *slog.Logger
}

// Gateway.
func NewGateway(args DiscordArguments) *Gateway {
	u := url.URL{
		Scheme:   "wss",
		Host:     "gateway.discord.gg",
		RawQuery: fmt.Sprintf("v=%v&encoding=json", 10),
	}
	return &Gateway{
		httpClient:     http.DefaultClient,
		wsDialer:       websocket.DefaultDialer,
		wsurl:          u.String(),
		botToken:       args.BotToken,
		botIntents:     args.BotIntent,
		botVersion:     10,
		status:         StatusDisconnected,
		discordBaseURL: "https://discord.com/api/",
		voiceManager:   voicemanager.NewVoiceManager(),
		log:            args.Logger,
	}
}

func (g *Gateway) Open(ctx context.Context) error {
	return g.open(ctx)
}

func (g *Gateway) open(ctx context.Context) error {
	g.log.Info("connecting to discord...")
	g.ctx = ctx
	var err error
	g.wsConn, _, err = g.wsDialer.DialContext(ctx, g.wsurl, nil)
	if err != nil {
		return err
	}

	_, rawMessage, err := g.wsConn.ReadMessage()
	if err != nil {
		return err
	}

	event := &structs.RawEvent{}
	if err := json.Unmarshal(rawMessage, event); err != nil {
		return err
	}

	if event.Op == OpcodeHello {
		d := new(structs.HelloEvent)
		if err := json.Unmarshal(event.D, &d); err != nil {
			return err
		}
		go g.heartbeating(time.Duration(d.HeartbeatInterval))
	}

	identify := structs.Event{
		Op: OpcodeIdentify,
		D: structs.IdentifyEvent{
			Token:   g.botToken,
			Intents: g.botIntents,
			Properties: structs.IdentifyEventProperties{
				Os:      "ubuntu",
				Browser: "siren",
				Device:  "siren",
			},
		},
	}
	data, err := json.Marshal(identify)
	if err != nil {
		return err
	}
	err = g.wsConn.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return errors.New("failed to send identify event")
	}
	g.log.Info("identify event sent")

	e := &structs.RawEvent{}
	err = g.wsConn.ReadJSON(e)
	if err != nil {
		if e, ok := err.(*websocket.CloseError); ok {
			switch e.Code {
			case AuthenticationFailed:
				return ErrAuthenticationFailed
			case NotAuthenticated:
				return ErrNotAuthenticated
			case DecodeError:
				return ErrDecode
			default:
				return ErrUnknown
			}
		}
		return err
	}
	readyEvent := &structs.ReadyEvent{}
	if err := json.Unmarshal(e.D, readyEvent); err != nil {
		return err
	}

	if e.T == StatusReady {
		g.log.Info("gateway is ready")
		g.status = StatusReady
		g.resumeGatewayURL = readyEvent.ResumeGatewayURL
		g.sessionID = readyEvent.SessionID
		g.log.Info("event", "ready_event", readyEvent)
		go g.listen(g.wsConn)
	}
	return nil
}

func (g *Gateway) retry(fn func() error, max int) error {
	for attempts := 1; attempts <= max; attempts++ {
		err := fn()
		if err == nil {
			return nil
		}
		g.log.Error("error occured. retrying...")
		delay := time.Duration(math.Pow(2, float64((attempts-1)))*1000) * time.Millisecond
		select {
		case <-time.After(delay):
			continue
		case <-g.ctx.Done():
			return nil
		}
	}
	return errors.New("failed after several attempts")
}

func (g *Gateway) acceptEvent(messageType int, rawMessage []byte) (*structs.Event, error) {
	var err error
	reader := bytes.NewBuffer(rawMessage)

	var e structs.Event
	decoder := json.NewDecoder(reader)
	if err = decoder.Decode(&e); err != nil {
		return &e, err
	}

	switch e.Op {
	case OpcodeHeartbeat:
		sequence := g.sequence.Load()
		g.log.Info("sequence")
		heartbeatEvent := structs.Event{
			Op: OpcodeHeartbeat,
			D:  sequence,
		}
		data, _ := json.Marshal(heartbeatEvent)
		g.sendEvent(websocket.BinaryMessage, data)
	case OpcodeHeartbeatAck:
		g.log.Info("event", "heartbeat_acknowledge", e)
	case OpcodeReconnect:
		g.status = StatusDisconnected
		g.reconnect()
	case OpcodeDispatch:
		g.sequence.Store(e.S)
		g.log.Info("event", "dispatch_event", e)
	}
	return &e, nil
}

func (g *Gateway) reconnect() error {
	var err error
	rurl, err := url.Parse(g.resumeGatewayURL)
	if err != nil {
		return err
	}
	resumeUrl := url.URL{
		Scheme:   rurl.Scheme,
		Host:     rurl.Host,
		RawQuery: fmt.Sprintf("v=%v&encoding=json", g.botVersion),
	}
	g.wsConn, _, err = g.wsDialer.DialContext(g.ctx, resumeUrl.String(), nil)
	if err != nil {
		return err
	}

	seq := g.sequence.Load()
	resumeEvent := &structs.Event{
		Op: OpcodeResume,
		D: &structs.ResumeEvent{
			Token:     g.botToken,
			SessionID: g.sessionID,
			Seq:       seq,
		},
	}
	data, err := json.Marshal(resumeEvent)
	if err != nil {
		return err
	}
	err = g.sendEvent(websocket.BinaryMessage, data)
	if err != nil {
		return err
	}
	go g.listen(g.wsConn)
	return nil
}

func (g *Gateway) listen(conn *websocket.Conn) {
	for {
		select {
		case <-g.ctx.Done():
			g.log.Info("gateway stop listening.")
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
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				// @todo
				panic(err)
			}
			g.acceptEvent(messageType, message)
		}
	}
}

func (g *Gateway) heartbeating(dur time.Duration) {
	g.heartbeatTicker = time.NewTicker(dur * time.Millisecond)
	for {
		select {
		case <-g.ctx.Done():
			g.heartbeatTicker.Stop()
			g.log.Info("gateway heartbeating process stopped")
			return
		case <-g.heartbeatTicker.C:
			sequence := g.sequence.Load()
			data, err := json.Marshal(structs.Event{
				Op: OpcodeHeartbeat,
				D:  sequence,
			})
			if err != nil {
				g.log.Error("failed to send heartbeat event")
				continue
			}
			g.sendEvent(websocket.BinaryMessage, data)
			g.lastHeartbeatSent = g.getLocalTime()
			g.log.Info("gateway heartbeat event sent")
		}
	}
}

func (g *Gateway) close() {
	if g.heartbeatTicker != nil {
		g.heartbeatTicker.Stop()
		g.log.Info("gateway heartbeat ticker stopped.")
	}
	g.wsConn.Close()
	g.log.Info("gateway connection stopped.")
}

func (g *Gateway) sendEvent(messageType int, data []byte) error {
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
		g.log.Info("interaction callback sent.")
		return
	}
	g.log.Warn("failed to send callback interaction.")
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

// func (g *Gateway) handlePlayCmd(interaction *structs.Interaction) {
// 	i := new(structs.InteractionResponse)
// 	i.Type = structs.InteractionResponseTypeChannelMessageWithSource
// 	response, err := g.sendHTTPRequest(g.ctx,
// 		http.MethodGet,
// 		fmt.Sprintf("%s/v%s/guilds/%s/voice-states/%s",
// 			g.discordBaseURL,
// 			g.botVersion,
// 			interaction.GuildID,
// 			interaction.Member.User.ID),
// 		nil)
// 	if err != nil {
// 		i.Data.Content = "failed to get current voice state."
// 		g.sendCallback(interaction.Token, interaction.ID, i)
// 		return
// 	}
// 	if response.StatusCode == http.StatusNotFound {
// 		i.Data.Content = fmt.Sprintf("%s, Please join to a voice channel before using this command.",
// 			g.mentionUser(interaction.Member.User.ID))
// 		g.sendCallback(interaction.Token, interaction.ID, i)
// 		return
// 	}
// 	defer response.Body.Close()
// 	b, _ := io.ReadAll(response.Body)
// 	voiceState := new(structs.VoiceState)
// 	json.Unmarshal(b, voiceState)
// 	data := &structs.GatewayVoiceState{
// 		GuildID:   interaction.GuildID,
// 		ChannelID: voiceState.ChannelID,
// 		SelfMute:  false,
// 		SelfDeaf:  false,
// 	}
// 	g.sendEvent(websocket.TextMessage, OpcodeVoiceStateUpdate, data)
// }

func (g *Gateway) HandleVoiceStateUpdate(data structs.VoiceStateUpdateData) {
	if voice := g.voiceManager.Get(data.GuildID); voice != nil {
		if len(data.ChannelID) == 0 {
			g.voiceManager.Delete(data.GuildID)
		}
		return
	}
	nv := voice.NewVoice(voice.NewVoiceArguments{
		SessionID: data.SessionID,
		UserID:    data.UserID,
		ServerID:  data.GuildID, // Server ID = Guild ID.
	})
	g.voiceManager.Add(data.GuildID, nv)
	return
}

func (g *Gateway) HandleVoiceServerUpdate(data structs.VoiceServerUpdateData) {
	if v := g.voiceManager.Get(data.GuildID); v != nil {
		v.Open(g.ctx, voice.VoiceOpenArgs{
			VoiceGatewayURL: data.Endpoint,
			VoiceToken:      data.Token,
		})
	}
	return
}
