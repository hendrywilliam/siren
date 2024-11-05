package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hendrywilliam/siren/src/structs"
)

const (
	CloseGracePeriod time.Duration = 10 * time.Second
	GatewayIntents   uint64        = 641
)

func StartGateway(ctx context.Context) {
	gatewayAddr, ok := os.LookupEnv("DC_GATEWAY_ADDRESS")
	if !ok || len(gatewayAddr) == 0 {
		log.Fatal("dc_gateway_address is not provided")
	}
	dcApiVersion, ok := os.LookupEnv("DC_API_VERSION")
	if !ok || len(dcApiVersion) == 0 {
		log.Fatal("dc_api_version is not provided")
	}
	u := url.URL{
		Scheme:   "wss",
		Host:     gatewayAddr,
		RawQuery: fmt.Sprintf("v=%s&encoding=json", dcApiVersion),
	}
	session := NewSession()
	var err error
	session.WSConn, _, err = session.WSDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer session.WSConn.Close()

	helloEvent := new(structs.HelloEvent)
	err = session.WSConn.ReadJSON(helloEvent)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("[EVENT]: Received Hello Event.")

	ticker := time.NewTicker(time.Duration(helloEvent.D.HeartbeatInterval) * time.Millisecond)
	defer ticker.Stop()

	defer close(session.HeartbeatEvent)
	defer close(session.Event)

	go session.ReadInboundEvent(ctx)
	go session.HandleHeartbeatEvent(ctx, ticker)
	go session.HandleEvent(ctx)

	err = session.SendIdentifyEvent()
	if err != nil {
		log.Println("failed to send identify event")
	}
	for {
		select {
		case <-ctx.Done():
			log.Println("closing current connection")
			err := session.WSConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Fatal(err)
				return
			}
			select {
			case <-time.After(CloseGracePeriod):
				log.Println("connection gracefully ended.")
			}
		}
	}
}

func (s *Session) ReadInboundEvent(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := s.WSConn.ReadMessage()
			if err != nil {
				log.Fatal(err)
			}
			event := new(structs.Event)
			err = json.Unmarshal(message, event)
			if err != nil {
				log.Fatal("failed to unmarshall")
			}
			switch event.Op {
			case structs.GatewayOpcodeHeartbeat, structs.GatewayOpcodeHeartbeatAck:
				s.HeartbeatEvent <- event
			default:
				s.Event <- event
			}
		}
	}
}

func (s *Session) HandleHeartbeatEvent(ctx context.Context, ticker *time.Ticker) {
	heartbeatEvent := &structs.HeartbeatEvent{
		Op: structs.GatewayOpcodeHeartbeat,
	}
	for {
		select {
		case <-ticker.C:
			s.RWLock.Lock()
			heartbeatEvent.D = s.Sequence
			s.RWLock.Unlock()
			encodedHeartbeatEvent, err := json.Marshal(heartbeatEvent)
			if err != nil {
				log.Println(err)
			}
			s.RWLock.Lock()
			err = s.WSConn.WriteMessage(websocket.TextMessage, encodedHeartbeatEvent)
			if err != nil {
				log.Println(err)
			}
			s.LastHeartbeatSent = time.Now().UTC()
			log.Println("[EVENT]: Heartbeat Event sent.")
			s.RWLock.Unlock()
		case event := <-s.HeartbeatEvent:
			if event.Op == structs.GatewayOpcodeHeartbeat {
				s.RWLock.Lock()
				heartbeatEvent.D = s.Sequence
				s.RWLock.Unlock()
				encodedHeartbeatEvent, err := json.Marshal(heartbeatEvent)
				if err != nil {
					log.Println(err)
				}
				s.RWLock.Lock()
				err = s.WSConn.WriteMessage(websocket.TextMessage, encodedHeartbeatEvent)
				if err != nil {
					log.Fatal(err)
				}
				s.LastHeartbeatSent = time.Now().UTC()
				s.RWLock.Unlock()
				continue
			}
			if event.Op == structs.GatewayOpcodeHeartbeatAck {
				s.RWLock.Lock()
				s.LastHeartbeatAcknowledge = time.Now().UTC()
				s.RWLock.Unlock()
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Session) HandleEvent(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.Event:
			switch event.T {
			case structs.EventNameReady:
				s.HandleReadyEvent(event)
			case structs.EventNameInteractionCreate:
				s.HandleInteractionEvent(ctx, event)
			}
		}
	}
}

func (s *Session) SendIdentifyEvent() error {
	botToken := os.Getenv("DC_BOT_TOKEN")
	if len(botToken) == 0 {
		panic("provide dc_bot_token")
	}
	identifyEvent := structs.IdentifyEvent{
		Op: structs.GatewayOpcodeIdentify,
		D: structs.IdentifyEventD{
			Token:   botToken,
			Intents: GatewayIntents,
			Properties: structs.IdentifyEventDProperties{
				Os:      "ubuntu",
				Browser: "chrome",
				Device:  "refrigerator",
			},
		},
	}
	encodedIdentifyEvent, err := json.Marshal(identifyEvent)
	if err != nil {
		return err
	}
	s.RWLock.Lock()
	defer s.RWLock.Unlock()
	return s.WSConn.WriteMessage(websocket.TextMessage, encodedIdentifyEvent)
}

func (s *Session) SendInteractionCallback(ctx context.Context, i *structs.Interaction) {
	dcApiVersion := os.Getenv("DC_API_VERSION")
	ir := structs.InteractionResponse{
		Type: structs.InteractionResponseTypeChannelMessageWithSource,
		Data: structs.InteractionResponseDataMessage{
			Content: "hello world",
		},
	}
	rb, err := json.Marshal(ir)
	if err != nil {
		panic(err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://discord.com/api/v%s/interactions/%s/%s/callback",
			dcApiVersion,
			i.ID,
			i.Token),
		bytes.NewBuffer(rb))
	if err != nil {
		panic(err)
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, err := s.HTTPClient.Do(request)
	if response.StatusCode == http.StatusNoContent {
		log.Println("[EVENT]: Interaction Callback sent.")
		return
	}
}

func (s *Session) HandleReadyEvent(event *structs.Event) {
	log.Println("[EVENT]: Received Ready Event.")
	i := new(structs.ReadyEventD)
	data, err := json.Marshal(event.D)
	if err != nil {
		panic("failed to marshall")
	}
	err = json.Unmarshal(data, i)
	if err != nil {
		panic("error decoding json")
	}
	s.RWLock.Lock()
	defer s.RWLock.Unlock()
	s.ResumeGatewayURL = i.ResumeGatewayURL
	s.SessionID = i.SessionID
	s.Sequence = event.S
	return
}

func (s *Session) HandleInteractionEvent(ctx context.Context, event *structs.Event) {
	i := new(structs.Interaction)
	mpData, err := json.Marshal(event.D)
	if err != nil {
		panic("failed to marshall interaction data")
	}
	err = json.Unmarshal(mpData, i)
	if err != nil {
		panic("error decoding json")
	}
	s.RWLock.Lock()
	s.Sequence = event.S
	s.RWLock.Unlock()
	go s.SendInteractionCallback(ctx, i)
}
