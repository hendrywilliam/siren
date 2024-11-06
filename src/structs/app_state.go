package structs

import "sync"

type AppState struct {
	appStateMutex sync.RWMutex

	Channels map[string]interface{}
	Guilds   map[string]interface{}
	Members  map[string]*Member
}
