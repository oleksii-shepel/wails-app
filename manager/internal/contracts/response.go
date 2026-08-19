package contracts

type ResponseStatus string

const (
	ResponseStatusSuccess ResponseStatus = "SUCCESS"
	ResponseStatusError   ResponseStatus = "ERROR"
)

type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type AppResponse struct {
	CommandID string         `json:"commandId"`
	Status    ResponseStatus `json:"status"`
	Timestamp int64          `json:"timestamp"`
	Data      interface{}    `json:"data,omitempty"`
	Error     *ErrorDetail   `json:"error,omitempty"`
}