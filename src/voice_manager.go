package src

import (
	"sync"
)

type GuildID = string

type VoiceManager struct {
	mu           sync.Mutex
	activeVoices map[GuildID]*Voice
}

func NewVoiceManager() VoiceManager {
	return VoiceManager{
		activeVoices: make(map[string]*Voice),
	}
}

func (vm VoiceManager) Add(guildId GuildID, voice *Voice) {
	if voice := vm.Get(guildId); voice != nil {
		return
	}
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.activeVoices[guildId] = voice
	return
}

func (vm *VoiceManager) Delete(guildID GuildID) {
	if voice := vm.Get(guildID); voice == nil {
		return
	}
	vm.mu.Lock()
	defer vm.mu.Unlock()
	delete(vm.activeVoices, guildID)
	return
}

func (vm *VoiceManager) Get(guildID GuildID) *Voice {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.activeVoices[guildID]
}
