package routemux

import (
	"net/http"
)

// Map is a shortcut for map[string]any.
type Map map[string]any

// Middleware wraps an http.Handler to add request/response processing.
type Middleware func(http.Handler) http.Handler

// Handler is an HTTP handler function that returns an error.
type Handler func(w http.ResponseWriter, req *http.Request) error

// HandlerType allows registering either func(w, r) or func(w, r) error.
type HandlerType interface {
	~func(http.ResponseWriter, *http.Request) |
		~func(http.ResponseWriter, *http.Request) error
}

// ErrorHandler handles errors returned by route handlers.
type ErrorHandler func(w http.ResponseWriter, req *http.Request, err error) error

// defaultErrHandler writes the error message or defaults to 500 Internal Server Error.
var defaultErrHandler ErrorHandler = func(
	w http.ResponseWriter,
	req *http.Request,
	err error,
) error {
	if httpErr, ok := IsHttpError(err); ok {
		http.Error(w, httpErr.Message, httpErr.Status)
		return nil
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return nil
}

// statusRecorder captures the HTTP status code without writing the response body.
type statusRecorder struct {
	status int
}

func (r *statusRecorder) Header() http.Header {
	return make(http.Header)
}

func (r *statusRecorder) Write([]byte) (int, error) {
	return 0, nil
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
}
