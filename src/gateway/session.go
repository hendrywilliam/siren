package gateway

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hendrywilliam/siren/src/structs"
)

type Session struct {
	RWLock sync.RWMutex

	AppState interface{}

	RateLimiter interface{}

	ResumeGatewayURL string
	SessionID        string
	SessionType      string
	WSConn           *websocket.Conn
	WSDialer         *websocket.Dialer

	LastHeartbeatAcknowledge time.Time // utc
	LastHeartbeatSent        time.Time // utc
	Sequence                 uint64

	HTTPClient *http.Client
	UserAgent  string

	Event          chan *structs.Event
	HeartbeatEvent chan *structs.Event
}

func NewSession() *Session {
	return &Session{
		HTTPClient:     http.DefaultClient,
		WSDialer:       websocket.DefaultDialer,
		Event:          make(chan *structs.Event),
		HeartbeatEvent: make(chan *structs.Event),
	}
}
