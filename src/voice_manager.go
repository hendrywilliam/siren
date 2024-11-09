package src

import "sync"

type GuildID = string

type VoiceManager struct {
	mu           sync.Mutex
	activeVoices map[GuildID]*Voice
}

func NewVoiceManager() *VoiceManager {
	return &VoiceManager{
		activeVoices: make(map[string]*Voice),
	}
}

func (vm *VoiceManager) AddVoice(guildId GuildID, voice *Voice) {
	if voice := vm.GetVoice(guildId); voice != nil {
		return
	}
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.activeVoices[guildId] = voice
	return
}

func (vm *VoiceManager) UpdateVoice(guildID GuildID, voice *Voice) {

}

func (vm *VoiceManager) DeleteVoice(guildID GuildID) {
	if voice := vm.GetVoice(guildID); voice == nil {
		return
	}
	vm.mu.Lock()
	defer vm.mu.Unlock()
	delete(vm.activeVoices, guildID)
	return
}

func (vm *VoiceManager) GetVoice(guildID GuildID) *Voice {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.activeVoices[guildID]
}
