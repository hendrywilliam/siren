package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/hendrywilliam/siren/src/structs"
)

type VoiceAPI struct {
	rest RESTClient
}

func NewVoiceAPI(rest RESTClient) *VoiceAPI {
	return &VoiceAPI{
		rest: rest,
	}
}

// Routes
func (v *VoiceAPI) listVoiceRoute() (string, error) {
	lvURL, err := url.JoinPath(v.rest.URL(), "/voice/regions")
	if err != nil {
		return "", err
	}
	return lvURL, nil
}

func (v *VoiceAPI) getCurrentUserVoiceStateRoute(guildID string) (string, error) {
	userVoiceStateURL, err := url.JoinPath(v.rest.URL(), fmt.Sprintf("/guilds/%s/voice-states/@me", guildID))
	if err != nil {
		return "", err
	}
	return userVoiceStateURL, nil
}

func (v *VoiceAPI) getUserVoiceStateRoute(guildID string, userID string) (string, error) {
	userVoiceStateURL, err := url.JoinPath(v.rest.URL(), fmt.Sprintf("/guilds/%s/voice-states/%s", guildID, userID))
	if err != nil {
		return "", err
	}
	return userVoiceStateURL, nil
}

func (v *VoiceAPI) GetCurrentUserVoiceState(ctx context.Context, guildID string) (*structs.VoiceState, error) {
	var err error
	voiceStateURL, err := v.getCurrentUserVoiceStateRoute(guildID)
	if err != nil {
		return nil, err
	}
	res, err := v.rest.Get(ctx, voiceStateURL, nil, nil)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	userVoiceState := &structs.VoiceState{}
	if err := json.Unmarshal(data, userVoiceState); err != nil {
		return nil, err
	}
	return userVoiceState, nil
}

func (v *VoiceAPI) GetUserVoiceState(ctx context.Context, guildID string, userID string) (*structs.VoiceState, error) {
	var err error
	voiceStateURL, err := v.getUserVoiceStateRoute(guildID, userID)
	if err != nil {
		return nil, err
	}
	res, err := v.rest.Get(ctx, voiceStateURL, nil, nil)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	userVoiceState := &structs.VoiceState{}
	if err := json.Unmarshal(data, userVoiceState); err != nil {
		return nil, err
	}
	return userVoiceState, err
}
