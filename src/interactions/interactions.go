package interactions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hendrywilliam/siren/src/rest"
	"github.com/hendrywilliam/siren/src/structs"
)

// Interaction API.
// Provide methods to interact with "Interaction" event struct.
// Source: https://discord.com/developers/docs/interactions/receiving-and-responding
type InteractionAPI struct {
	rest rest.RESTClient
}

func New(rest rest.RESTClient) *InteractionAPI {
	return &InteractionAPI{rest: rest}
}

// Routes
// Sources: https://discord.com/developers/docs/interactions/receiving-and-responding
func (i *InteractionAPI) interactionResponseCallbackRoute(interactionID string, interactionToken string, withResponse bool) (string, error) {
	u, err := url.Parse(i.rest.URL())
	if err != nil {
		return "", err
	}
	cbPath := fmt.Sprintf("/interactions/%s/%s/callback", interactionID, interactionToken)
	actualPath, err := url.JoinPath(u.Path, cbPath)
	if err != nil {
		return "", err
	}
	cbUrl := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   actualPath,
	}
	queries := u.Query()
	if withResponse {
		queries.Add("with_response", "true")
	}
	cbUrl.RawQuery = queries.Encode()
	return cbUrl.String(), nil
}

func (i *InteractionAPI) originalInteractionRoute(applicationID, interactionToken, threadID string, withComponents bool) (string, error) {
	u, err := url.Parse(i.rest.URL())
	if err != nil {
		return "", err
	}
	orgPath := fmt.Sprintf("/webhooks/%s/%s/messages/@original", applicationID, interactionToken)
	actualPath, err := url.JoinPath(u.Path, orgPath)
	if err != nil {
		return "", err
	}
	orgUrl := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   actualPath,
	}
	q := u.Query()
	if threadID != "" {
		q.Add("thread_id", threadID)
	}
	if withComponents {
		// default false
		q.Add("with_components", "true")
	}
	orgUrl.RawQuery = q.Encode()
	return orgUrl.String(), nil
}

type CreateInteractionResponseOptions struct {
	InteractionResponse *structs.InteractionResponse
	WithResponse        bool
}

// Methods
func (i *InteractionAPI) Reply(ctx context.Context, interactionID string, interactionToken string, options CreateInteractionResponseOptions) (*http.Response, error) {
	var err error
	cbURL, err := i.interactionResponseCallbackRoute(interactionID, interactionToken, options.WithResponse)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = json.NewEncoder(buf).Encode(options.InteractionResponse)
	if err != nil {
		return nil, err
	}
	res, err := i.rest.Post(ctx, cbURL, buf, nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type EditOriginalData struct {
	Content         string `json:"string"`
	Flags           int32  `json:"flags"`
	PayloadJson     string `json:"payload_json"`
	Attachments     any    `json:"attachments"`      // unimplemented
	Poll            any    `json:"poll"`             // unimplemented
	Embed           any    `json:"embed"`            // unimplemented
	AllowedMentions any    `json:"allowed_mentions"` // unimplemented
	Components      any    `json:"components"`       // unimplemented
	Files           any    `json:"files"`            // unimplemented
}

type EditOriginalOptions struct {
	Data           EditOriginalData
	ThreadID       string
	WithComponents bool
}

func (i *InteractionAPI) EditOriginal(ctx context.Context, applicationID, interactionToken string, options EditOriginalOptions) (*http.Response, error) {
	var err error
	orgURL, err := i.originalInteractionRoute(applicationID, interactionToken, options.ThreadID, options.WithComponents)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(options.Data); err != nil {
		return nil, err
	}
	res, err := i.rest.Patch(ctx, orgURL, buf, nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type GetOriginalOptions struct {
	ThreadID string
}

func (i *InteractionAPI) GetOriginal(ctx context.Context, applicationID, interactionToken string, options GetOriginalOptions) (*http.Response, error) {
	var err error
	orgURL, err := i.originalInteractionRoute(applicationID, interactionToken, options.ThreadID, false)
	if err != nil {
		return nil, err
	}
	res, err := i.rest.Get(ctx, orgURL, nil, nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (i *InteractionAPI) DeleteOriginal(ctx context.Context, applicationID string, interactionToken string) (*http.Response, error) {
	var err error
	orgURL, err := i.originalInteractionRoute(applicationID, interactionToken, "", false)
	if err != nil {
		return nil, err
	}
	res, err := i.rest.Delete(ctx, orgURL, nil, nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}
