package formatters

import (
	"encoding/json"
	"net/http"

	"github.com/FBDAF/microservices/services/common/errors/types"
)

// ErrorResponse represents the standard REST error response format
type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
	Timestamp string `json:"timestamp"`
	TraceID   string `json:"trace_id,omitempty"`
}

// FormatError formats an error into a standard REST error response
func FormatError(err error) (int, ErrorResponse) {
	if err == nil {
		return http.StatusOK, ErrorResponse{}
	}

	// Default to internal server error
	statusCode := http.StatusInternalServerError
	response := ErrorResponse{
		Code:      string(types.ErrCodeInternal),
		Message:   err.Error(),
		Severity:  string(types.SeverityHigh),
		Timestamp: "",
	}

	// If it's our custom error type, use its properties
	if customErr, ok := err.(types.Error); ok {
		response.Code = string(customErr.Code())
		response.Severity = string(customErr.Severity())
		response.Timestamp = customErr.Timestamp().Format("2006-01-02T15:04:05Z07:00")
		response.TraceID = customErr.TraceID()

		// Map error codes to HTTP status codes
		switch customErr.Code() {
		case types.ErrCodeValidation:
			statusCode = http.StatusBadRequest
		case types.ErrCodeNotFound:
			statusCode = http.StatusNotFound
		case types.ErrCodeUnauthorized:
			statusCode = http.StatusUnauthorized
		case types.ErrCodeForbidden:
			statusCode = http.StatusForbidden
		case types.ErrCodeTimeout:
			statusCode = http.StatusGatewayTimeout
		case types.ErrCodeConflict:
			statusCode = http.StatusConflict
		}
	}

	return statusCode, response
}

// WriteError writes an error response to an HTTP response writer
func WriteError(w http.ResponseWriter, err error) {
	statusCode, response := FormatError(err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// If we can't encode the error response, write a basic error
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"Failed to encode error response"}`))
	}
}
