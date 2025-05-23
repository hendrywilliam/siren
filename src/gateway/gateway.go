package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hendrywilliam/siren/src/api"
	"github.com/hendrywilliam/siren/src/structs"
	"github.com/hendrywilliam/siren/src/voice"
	"github.com/hendrywilliam/siren/src/voicemanager"
)

// https://discord.com/developers/docs/events/gateway#message-content-intent
type GatewayIntent = int

var (
	GuildsIntent                      = 1 << 0
	GuildMembersIntent                = 1 << 1
	GuildModerationIntent             = 1 << 2
	GuildExpressionIntent             = 1 << 3
	GuildIntegrationsIntent           = 1 << 4
	GuildWebhooksIntent               = 1 << 5
	GuildInvitesIntent                = 1 << 6
	GuildVoiceStatesIntent            = 1 << 7
	GuildPresencesIntent              = 1 << 8
	GuildMessagesIntent               = 1 << 9
	GuildMessageReactionIntent        = 1 << 10
	GuildMessageTypingIntent          = 1 << 11
	DirectMessageIntent               = 1 << 12
	DirectMessageReactionIntent       = 1 << 13
	DirectMessageTypingIntent         = 1 << 14
	MessageContentIntent              = 1 << 15
	GuildScheduledEventsIntent        = 1 << 16
	AutoModerationConfigurationIntent = 1 << 20
	AutoModerationExecutionIntent     = 1 << 21
	GuildMessagePollsIntent           = 1 << 24
	DirectMessagePollsIntent          = 1 << 25
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
	UnknownError         GatewayCloseEventCode = 4000
	UnknownOpcode        GatewayCloseEventCode = 4001
	DecodeError          GatewayCloseEventCode = 4002
	NotAuthenticated     GatewayCloseEventCode = 4003
	AuthenticationFailed GatewayCloseEventCode = 4004
	AlreadyAuthenticated GatewayCloseEventCode = 4005
	InvalidSeq           GatewayCloseEventCode = 4007
	RateLimited          GatewayCloseEventCode = 4008
	SessionTimedOut      GatewayCloseEventCode = 4009

	DisallowedIntents GatewayCloseEventCode = 4014
)

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrNotAuthenticated     = errors.New("not authenticated")
	ErrDecode               = errors.New("invalid payload")
	ErrGatewayIsAlreadyOpen = errors.New("gateway is already open")
	ErrUnknown              = errors.New("unknown error")
	ErrDisallowedIntents    = errors.New("disallowed intent. you may have tried to specify an intent that you have not enabled")
	ErrUnrecognizedEvent    = errors.New("error unrecognized event.")
)

type Gateway struct {
	rwlock           sync.RWMutex
	wsurl            string
	resumeGatewayURL string
	sessionID        string
	wsConn           *websocket.Conn
	wsDialer         *websocket.Dialer
	sequence         atomic.Uint64
	ctx              context.Context
	heartbeatTicker  *time.Ticker
	status           GatewayStatus

	botToken           string
	botIntents         int
	botVersion         uint
	clientID           string
	discordHTTPBaseURL string

	voiceManager voicemanager.VoiceManager
	log          *slog.Logger
	// APIs
	rest        *api.REST
	interaction *api.InteractionAPI
	message     *api.MessageAPI
	voice       *api.VoiceAPI
}

type DiscordArguments struct {
	BotToken   string
	BotIntent  []int
	BotVersion uint
	ClientID   string

	Logger *slog.Logger
}

// Gateway.
func NewGateway(args DiscordArguments) *Gateway {
	// https://discord.com/developers/docs/reference#http-api
	wsBaseURL := url.URL{
		Scheme:   "wss",
		Host:     "gateway.discord.gg",
		RawQuery: fmt.Sprintf("v=%d&encoding=json", args.BotVersion),
	}
	httpBaseURL := url.URL{
		Scheme: "https",
		Host:   "discord.com",
		Path:   fmt.Sprintf("api/v%d", args.BotVersion),
	}

	intents := 0
	for _, v := range args.BotIntent {
		intents += v
	}

	// APIs
	restAPI := api.NewREST(httpBaseURL.String(), args.BotToken)
	interactionAPI := api.NewInteractionAPI(restAPI)
	messageAPI := api.NewMessageAPI(restAPI)
	voiceAPI := api.NewVoiceAPI(restAPI)

	return &Gateway{
		clientID:           args.ClientID,
		wsDialer:           websocket.DefaultDialer,
		wsurl:              wsBaseURL.String(),
		botToken:           args.BotToken,
		botIntents:         intents,
		botVersion:         args.BotVersion,
		status:             StatusDisconnected,
		discordHTTPBaseURL: httpBaseURL.String(),
		voiceManager:       voicemanager.NewVoiceManager(),
		log:                args.Logger,
		rest:               restAPI,
		interaction:        interactionAPI,
		message:            messageAPI,
		voice:              voiceAPI,
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

	err = g.sendEvent(websocket.BinaryMessage, data)
	if err != nil {
		return errors.New("failed to send identify event")
	}
	// g.log.Info("identify event sent")

	e := &structs.RawEvent{}
	err = g.wsConn.ReadJSON(e)
	if err != nil {
		g.log.Error(err.Error())
		if e, ok := err.(*websocket.CloseError); ok {
			switch e.Code {
			case AuthenticationFailed:
				return ErrAuthenticationFailed
			case NotAuthenticated:
				return ErrNotAuthenticated
			case DecodeError:
				return ErrDecode
			case DisallowedIntents:
				return ErrDisallowedIntents
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
		g.status = StatusReady
		g.resumeGatewayURL = readyEvent.ResumeGatewayURL
		g.sessionID = readyEvent.SessionID
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

func (g *Gateway) isSelf(id string) bool {
	return id == g.clientID
}

func (g *Gateway) acceptEvent(messageType int, rawMessage []byte) (*structs.RawEvent, error) {
	var err error
	reader := bytes.NewBuffer(rawMessage)

	e := &structs.RawEvent{}
	decoder := json.NewDecoder(reader)
	if err = decoder.Decode(&e); err != nil {
		return e, err
	}

	switch e.Op {
	case OpcodeHeartbeat:
		sequence := g.sequence.Load()
		heartbeatEvent := structs.Event{
			Op: OpcodeHeartbeat,
			D:  sequence,
		}
		data, _ := json.Marshal(heartbeatEvent)
		g.sendEvent(websocket.BinaryMessage, data)
		return e, nil
	case OpcodeHeartbeatAck:
		g.log.Info("event", "heartbeat_acknowledge", e)
		return e, nil
	case OpcodeReconnect:
		g.status = StatusDisconnected
		err := g.reconnect()
		if err != nil {
			return nil, err
		}
		return e, nil
	case OpcodeDispatch:
		err := g.onEvent(*e)
		if err != nil {
			return nil, err
		}
		return e, nil
	default:
		return nil, ErrUnrecognizedEvent
	}
}

func (g *Gateway) onEvent(e structs.RawEvent) error {
	g.sequence.Store(e.S)
	switch e.T {
	case "MESSAGE_CREATE":
		messageEvent := structs.Message{}
		if err := json.Unmarshal(e.D, &messageEvent); err != nil {
			return err
		}
		if g.isSelf(messageEvent.Author.ID) {
			return nil
		}

		// Create message
		_, err := g.message.CreateMessage(g.ctx, messageEvent.ChannelID, api.CreateMessageOptions{
			Data: api.CreateMessageData{
				Content: fmt.Sprintf("hello, %s", messageEvent.Author.Mention()),
			},
		})
		if err != nil {
			return err
		}
	case "INTERACTION_CREATE":
		interactionEvent := structs.Interaction{}
		if err := json.Unmarshal(e.D, &interactionEvent); err != nil {
			return err
		}
		// g.log.Info("event", "interaction_create", interactionEvent)

		// Check user voice state
		userVoiceState, err := g.voice.GetUserVoiceState(g.ctx, interactionEvent.GuildID, interactionEvent.Member.User.ID)
		if err != nil {
			return err
		}

		// User hasn't joined to a voice channel.
		if userVoiceState.SessionID == "" {
			_, err := g.interaction.Reply(g.ctx, interactionEvent.ID, interactionEvent.Token, api.CreateInteractionResponseOptions{
				InteractionResponse: &structs.InteractionResponse{
					Type: structs.InteractionResponseTypeChannelMessageWithSource,
					Data: structs.InteractionResponseDataMessage{
						Content: fmt.Sprintf("%s, join to a voice channel first.", interactionEvent.Member.User.Mention()),
					},
				},
			})
			if err != nil {
				return err
			}
			return nil
		}
		_, err = g.interaction.Reply(g.ctx, interactionEvent.ID, interactionEvent.Token, api.CreateInteractionResponseOptions{
			InteractionResponse: &structs.InteractionResponse{
				Type: structs.InteractionResponseTypeChannelMessageWithSource,
				Data: structs.InteractionResponseDataMessage{
					Content: fmt.Sprintf("Playing 'Sunflower' for %s", interactionEvent.Member.User.Mention()),
				},
			},
			WithResponse: false,
		})
		if err != nil {
			return err
		}

		// Send voice state update
		voiceStateUpdate := &structs.Event{
			Op: OpcodeVoiceStateUpdate,
			D: &structs.VoiceStateUpdate{
				GuildID:   userVoiceState.GuildID,
				ChannelID: userVoiceState.ChannelID,
				SelfMute:  userVoiceState.SelfMute,
				SelfDeaf:  userVoiceState.SelfDeaf,
			},
		}
		// g.log.Info("event", "voice_state_update", voiceStateUpdate)
		data, err := json.Marshal(voiceStateUpdate)
		if err != nil {
			return err
		}
		g.sendEvent(websocket.BinaryMessage, data)
	case "VOICE_STATE_UPDATE":
		voiceStateEvent := structs.VoiceState{}
		if err := json.Unmarshal(e.D, &voiceStateEvent); err != nil {
			return err
		}
		g.log.Info("event", "voice_state_update", voiceStateEvent)
		// Create new voice instance
		newVoice := voice.NewVoice(voice.NewVoiceArguments{
			SessionID:  voiceStateEvent.SessionID,
			ServerID:   voiceStateEvent.GuildID,
			BotVersion: g.botVersion,
			UserID:     voiceStateEvent.UserID,
			Log:        g.log,
		})
		g.voiceManager.Add(voiceStateEvent.GuildID, newVoice)
	case "VOICE_SERVER_UPDATE":
		voiceUpdateEvent := structs.VoiceServerUpdate{}
		if err := json.Unmarshal(e.D, &voiceUpdateEvent); err != nil {
			return err
		}
		g.log.Info("event", "voice_server_update", voiceUpdateEvent)
		voice := g.voiceManager.Get(voiceUpdateEvent.GuildID)
		voice.VoiceGatewayURL = voiceUpdateEvent.Endpoint
		voice.Token = voiceUpdateEvent.Token
		voice.Open(g.ctx)
	}

	return nil
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

func (g *Gateway) heartbeating(dur time.Duration) error {
	g.heartbeatTicker = time.NewTicker(dur * time.Millisecond)
	for {
		select {
		case <-g.ctx.Done():
			g.heartbeatTicker.Stop()
			g.log.Info("gateway heartbeating process stopped")
			return nil
		case <-g.heartbeatTicker.C:
			sequence := g.sequence.Load()
			data, err := json.Marshal(structs.Event{
				Op: OpcodeHeartbeat,
				D:  sequence,
			})
			if err != nil {
				g.log.Error("failed to send heartbeat event")
				return err
			}
			g.sendEvent(websocket.BinaryMessage, data)
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
