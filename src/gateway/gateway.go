package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	CloseGracePeriod time.Duration = 10 * time.Second
)

func StartGateway(ctx context.Context) {
	gatewayAddr, ok := os.LookupEnv("DC_GATEWAY_ADDRESS")
	if !ok || len(gatewayAddr) == 0 {
		log.Fatal().Msg("dc_gateway_address is not provided")
	}
	dcApiVersion, ok := os.LookupEnv("DC_API_VERSION")
	if !ok || len(dcApiVersion) == 0 {
		log.Fatal().Msg("dc_api_version is not provided")
	}
	u := url.URL{
		Scheme:   "wss",
		Host:     gatewayAddr,
		RawQuery: fmt.Sprintf("v=%s&encoding=json", dcApiVersion),
	}
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		log.Error().Err(err)
	}
	defer ws.Close()

	helloEvent := new(HelloEvent)
	err = ws.ReadJSON(helloEvent)
	if err != nil {
		log.Error().Err(err)
	}
	log.Info().Msg(fmt.Sprintf("received hello event: %v", helloEvent))

	ticker := time.NewTicker(time.Duration(helloEvent.D.HeartbeatInterval) * time.Millisecond)
	defer ticker.Stop()

	heartbeatEventChan := make(chan Event)
	defer close(heartbeatEventChan)

	go ReadInboundEvent(ctx, ws, heartbeatEventChan)
	go SendHeartbeatEvent(ctx, ws, ticker, heartbeatEventChan)

	err = SendIdentifyEvent(ws)
	if err != nil {
		log.Error().Err(err).Msg("failed to send identify event")
	}

	for {
		select {
		case <-ctx.Done():
			err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Error().Err(err)
				return
			}
			select {
			case <-time.After(CloseGracePeriod):
			}
		}
	}
}

func ReadInboundEvent(ctx context.Context, ws *websocket.Conn, heartbeatChan chan<- Event) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := ws.ReadMessage()
			if err != nil {
				log.Error().Err(err)
			}
			event := new(Event)
			err = json.Unmarshal(message, event)
			if err != nil {
				log.Error().Err(err).Msg("failed to unmarshall")
			}
			switch event.Op {
			case 1:
				log.Info().Msg("heartbeat event")
				heartbeatChan <- *event
			default:
				log.Info().Msg(fmt.Sprintf("[event]: %v", event))
			}
		}
	}
}

func SendHeartbeatEvent(ctx context.Context, ws *websocket.Conn, ticker *time.Ticker, heartbeatChan <-chan Event) {
	heartbeatEvent := Event{
		Op: HeartbeatSendOpcode,
		D:  251, // numeros magicos
	}
	encodedHeartbeatEvent, err := json.Marshal(heartbeatEvent)
	if err != nil {
		log.Error().Err(err).Msg("failed to encode heartbeat event.")
	}
	for {
		select {
		case <-ticker.C:
			err = ws.WriteMessage(websocket.TextMessage, encodedHeartbeatEvent)
			if err != nil {
				log.Error().Err(err)
				return
			}
		case heartbeatEvent := <-heartbeatChan:
			if heartbeatEvent.Op == 1 {
				err = ws.WriteMessage(websocket.TextMessage, encodedHeartbeatEvent)
				if err != nil {
					log.Error().Err(err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func SendIdentifyEvent(ws *websocket.Conn) error {
	botToken := os.Getenv("DC_BOT_TOKEN")
	if len(botToken) == 0 {
		panic("provide dc_bot_token")
	}
	identifyEvent := IdentifyEvent{
		Op: IdentifySendOpcode,
		D: IdentifyEventD{
			Token:   botToken,
			Intents: 641,
			Properties: IdentifyEventDProperties{
				Os:      "ubuntu",
				Browser: "chrome",
				Device:  "pc",
			},
		},
	}
	encodedIdentifyEvent, err := json.Marshal(identifyEvent)
	if err != nil {
		return err
	}
	err = ws.WriteMessage(websocket.TextMessage, encodedIdentifyEvent)
	return err
}
