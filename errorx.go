package routemux

import (
	"errors"
	"net/http"
)

// HttpError represents an HTTP error response.
type HttpError struct {
	Status  int            `json:"status"`
	Name    string         `json:"error"`
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message"`
	Meta    map[string]any `json:"meta,omitempty"`
	cause   error
}

// Option configures an HttpError.
type Option func(*HttpError)

// WithCause attaches the underlying error.
func WithCause(err error) Option {
	return func(e *HttpError) {
		e.cause = err
	}
}

// WithMeta attaches custom metadata key-value pairs to the error.
func WithMeta(key string, value any) Option {
	return func(e *HttpError) {
		if e.Meta == nil {
			e.Meta = make(map[string]any)
		}
		e.Meta[key] = value
	}
}

// NewError creates an HttpError with status code, custom code, and message.
func NewError(
	status int,
	code string,
	message string,
	opts ...Option,
) *HttpError {
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	err := &HttpError{
		Status:  status,
		Code:    code,
		Message: message,
		Name:    http.StatusText(status),
		Meta:    make(map[string]any),
	}
	for _, opt := range opts {
		opt(err)
	}
	return err
}

// NewHttpError creates an HttpError with status and message.
func NewHttpError(status int, message string, opts ...Option) *HttpError {
	return NewError(status, "", message, opts...)
}

// Error returns the error message.
func (e *HttpError) Error() string {
	return e.Message
}

// Cause returns the underlying root cause error.
func (e *HttpError) Cause() error {
	return e.cause
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *HttpError) Unwrap() error {
	return e.cause
}

// ToJSON writes the HttpError as a JSON response.
func (e *HttpError) ToJSON(w http.ResponseWriter) error {
	return JSON(w, e.Status, e)
}

// IsHttpError checks if err is or wraps an *HttpError.
func IsHttpError(err error) (*HttpError, bool) {
	if err == nil {
		return nil, false
	}
	httpErr, ok := errors.AsType[*HttpError](err)
	return httpErr, ok
}

// create returns a constructor for a specific HTTP status code.
func create(status int) func(message string, code string, opts ...Option) *HttpError {
	return func(message string, code string, opts ...Option) *HttpError {
		return NewError(status, code, message, opts...)
	}
}

// Common HTTP error constructors.
//
// Example:
//
//	return mux.BadRequestError("invalid email", "INVALID_EMAIL")
//	return mux.NotFoundError("user not found", "USER_NOT_FOUND")
var (
	BadRequestError      = create(http.StatusBadRequest)
	ConflictError        = create(http.StatusConflict)
	ForbiddenError       = create(http.StatusForbidden)
	NotFoundError        = create(http.StatusNotFound)
	UnauthorizedError    = create(http.StatusUnauthorized)
	InternalServerError  = create(http.StatusInternalServerError)
	ContentTooLargeError = create(http.StatusRequestEntityTooLarge)
)
