package src

type HTTPErrorCode = uint

const (
	HTTPErrorUnknownVoiceState HTTPErrorCode = 10065
)

// http response when interacting to discord's resources
type ErrorHTTPResponse struct {
	Message string      `json:"message"`
	Code    uint        `json:"code"`
	Errors  interface{} `json:"errors,omitempty"`
}
