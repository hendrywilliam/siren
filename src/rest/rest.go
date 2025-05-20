package rest

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type REST struct {
	httpClient *http.Client
	botToken   string
}

type RESTOptions struct {
	Headers map[string]string
}

func NewREST(botToken string) *REST {
	r := &REST{
		httpClient: http.DefaultClient,
		botToken:   botToken,
	}
	return r
}

func (r *REST) applyHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func (r *REST) makeRequest(ctx context.Context, method string, url string, body io.Reader, options *RESTOptions) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	// Mandatory headers.
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", r.botToken))

	if options != nil {
		// Apply all options in here, including additional headers.
		r.applyHeaders(req, options.Headers)
	}
	return req, nil
}

func (r *REST) Get(ctx context.Context, url string, body io.Reader, options *RESTOptions) (*http.Response, error) {
	req, err := r.makeRequest(ctx, http.MethodGet, url, body, options)
	if err != nil {
		return nil, err
	}
	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *REST) Put(ctx context.Context, url string, body io.Reader, options *RESTOptions) (*http.Response, error) {
	req, err := r.makeRequest(ctx, http.MethodPut, url, body, options)
	if err != nil {
		return nil, err
	}
	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *REST) Patch(ctx context.Context, url string, body io.Reader, options *RESTOptions) (*http.Response, error) {
	req, err := r.makeRequest(ctx, http.MethodPatch, url, body, options)
	if err != nil {
		return nil, err
	}
	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *REST) Delete(ctx context.Context, url string, body io.Reader, options *RESTOptions) (*http.Response, error) {
	req, err := r.makeRequest(ctx, http.MethodDelete, url, body, options)
	if err != nil {
		return nil, err
	}
	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *REST) Post(ctx context.Context, url string, body io.Reader, options *RESTOptions) (*http.Response, error) {
	req, err := r.makeRequest(ctx, http.MethodPost, url, body, options)
	if err != nil {
		return nil, err
	}
	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
