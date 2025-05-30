package transport

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler defines the interface for HTTP handlers
type Handler interface {
	// RegisterRoutes registers the handler routes with the Gin router
	RegisterRoutes(router *gin.Engine)
}

// Middleware defines the interface for Gin middleware
type Middleware interface {
	// Handler returns a Gin middleware handler function
	Handler() gin.HandlerFunc
}

// Server defines the interface for HTTP servers
type Server interface {
	// Start starts the server
	Start() error

	// Stop stops the server
	Stop(ctx context.Context) error

	// AddHandler adds a handler to the server
	AddHandler(handler Handler)

	// AddMiddleware adds middleware to the server
	AddMiddleware(middleware Middleware)
}

// Client defines the interface for HTTP clients
type Client interface {
	// Do performs an HTTP request
	Do(*http.Request) (*http.Response, error)

	// Get performs a GET request
	Get(url string) (*http.Response, error)

	// Post performs a POST request
	Post(url, contentType string, body interface{}) (*http.Response, error)

	// Put performs a PUT request
	Put(url, contentType string, body interface{}) (*http.Response, error)

	// Delete performs a DELETE request
	Delete(url string) (*http.Response, error)
}

// Response represents a standardized HTTP response
type Response struct {
	Status  int         `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewResponse creates a new Response
func NewResponse(status int, message string, data interface{}, err error) *Response {
	resp := &Response{
		Status:  status,
		Message: message,
		Data:    data,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

// ErrorResponse creates a standardized error response
func ErrorResponse(c *gin.Context, status int, err error) {
	c.JSON(status, NewResponse(status, "", nil, err))
}

// SuccessResponse creates a standardized success response
func SuccessResponse(c *gin.Context, status int, data interface{}) {
	c.JSON(status, NewResponse(status, "success", data, nil))
}
