package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/siren/interactions"
)

const (
	CloseGracePeriod time.Duration = 10 * time.Second
	GatewayIntents   uint64        = 641
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

	heartbeatEventChan := make(chan *Event)
	defer close(heartbeatEventChan)
	genericEventChan := make(chan *Event)
	defer close(genericEventChan)

	go ReadInboundEvent(ctx, ws, heartbeatEventChan, genericEventChan)
	go SendHeartbeatEvent(ctx, ws, ticker, heartbeatEventChan)
	go SendCallbackInteraction(ctx, ws, genericEventChan)

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
			// unimplemented
			case <-time.After(CloseGracePeriod):
			}
		}
	}
}

func ReadInboundEvent(ctx context.Context, ws *websocket.Conn, heartbeatChan chan<- *Event, genericEventChan chan<- *Event) {
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
				log.Panic().Err(err).Msg("failed to unmarshall")
			}
			switch event.Op {
			case 1:
				heartbeatChan <- event
			default:
				genericEventChan <- event
			}
		}
	}
}

func SendHeartbeatEvent(ctx context.Context, ws *websocket.Conn, ticker *time.Ticker, heartbeatChan <-chan *Event) {
	heartbeatEvent := Event{
		Op: GatewayOpcodeHeartbeat,
		D:  251,
	}
	encodedHeartbeatEvent, err := json.Marshal(heartbeatEvent)
	if err != nil {
		log.Error().Err(err)
	}
	for {
		select {
		case <-ticker.C:
			err = ws.WriteMessage(websocket.TextMessage, encodedHeartbeatEvent)
			if err != nil {
				log.Error().Err(err)
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

func SendCallbackInteraction(ctx context.Context, ws *websocket.Conn, genericEventChan <-chan *Event) {
	dcApiVersion := os.Getenv("DC_API_VERSION")
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-genericEventChan:
			// should create constant string later
			if event.T == "INTERACTION_CREATE" {
				if interactionData, ok := event.D.(map[string]interface{}); ok {
					i := new(interactions.Interaction)
					mpData, err := json.Marshal(interactionData)
					if err != nil {
						panic("failed to marshall interaction data")
					}
					err = json.Unmarshal(mpData, i)
					if err != nil {
						panic("error decoding json")
					}
					go func() {
						interactionResponse := interactions.InteractionResponse{
							Type: interactions.InteractionResponseTypeChannelMessageWithSource,
							Data: interactions.InteractionResponseDataMessage{
								Content: "hello world",
							},
						}
						rb, err := json.Marshal(interactionResponse)
						if err != nil {
							log.Error().Err(err)
						}
						request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://discord.com/api/v%s/interactions/%s/%s/callback", dcApiVersion, i.ID, i.Token), bytes.NewBuffer(rb))
						if err != nil {
							log.Fatal().Err(err)
						}
						request.Header.Set("Content-Type", "application/json; charset=UTF-8")
						httpClient := &http.Client{}

						// todo - should handle exception
						response, err := httpClient.Do(request)
						if err != nil {
							log.Fatal().Err(err)
						}
						defer response.Body.Close()
						body, _ := io.ReadAll(response.Body)
						log.Info().Msg(string(body))
					}()
				}
			}
		}
	}
}

func SendIdentifyEvent(ws *websocket.Conn) error {
	botToken := os.Getenv("DC_BOT_TOKEN")
	if len(botToken) == 0 {
		panic("provide dc_bot_token")
	}
	identifyEvent := IdentifyEvent{
		Op: GatewayOpcodeIdentify,
		D: IdentifyEventD{
			Token:   botToken,
			Intents: GatewayIntents,
			Properties: IdentifyEventDProperties{
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
	err = ws.WriteMessage(websocket.TextMessage, encodedIdentifyEvent)
	return err
}
