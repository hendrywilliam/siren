package message

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/hendrywilliam/siren/src/rest"
	"github.com/hendrywilliam/siren/src/structs"
)

// Messages API.
// Provide methods to interact with "Messages" event struct.
// Source: https://discord.com/developers/docs/resources/message
type MessageAPI struct {
	rest rest.RESTClient
}

func New(rest rest.RESTClient) *MessageAPI {
	return &MessageAPI{
		rest: rest,
	}
}

// Routes
func (m *MessageAPI) createMessageRoute(channelID string) (string, error) {
	u, err := url.Parse(m.rest.URL())
	if err != nil {
		return "", err
	}
	cmPath := fmt.Sprintf("/channels/%s/messages", channelID)
	actualPath, err := url.JoinPath(u.Path, cmPath)
	if err != nil {
		return "", err
	}
	cmURL := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   actualPath,
	}
	return cmURL.String(), nil
}

type CreateMessageData struct {
	Content          string `json:"content"`
	Tts              bool   `json:"tts"`
	Nonce            any    `json:"nonce,omitempty"`             // Use nonce to verify a message was sent.
	Embeds           any    `json:"embeds,omitempty"`            // unimplemented
	AllowedMentions  any    `json:"allowed_mentions,omitempty"`  // unimplemented
	MessageReference any    `json:"message_reference,omitempty"` // unimplemented
	Components       any    `json:"components,omitempty"`        // unimplemented
	StickerIDS       any    `json:"sticker_ids,omitempty"`       // unimplemented
}

type CreateMessageOptions struct {
	Data CreateMessageData
}

func (m *MessageAPI) CreateMessage(ctx context.Context, channelID string, options CreateMessageOptions) (*structs.Message, error) {
	var err error
	cmURL, err := m.createMessageRoute(channelID)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(options.Data); err != nil {
		return nil, err
	}
	res, err := m.rest.Post(ctx, cmURL, buf, nil)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	msg := &structs.Message{}
	if err := json.Unmarshal(b, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
