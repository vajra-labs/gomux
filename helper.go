package gomux

import (
	"context"
	"encoding/json/v2"
	"io"
	"net/http"
)

type userCtxKey struct{ name string }

// Set stores a key-value pair in the request context and returns the updated *http.Request.
//
// Example:
//
//	r = mux.Set(r, "userID", "123")
func Set(r *http.Request, key string, val any) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userCtxKey{name: key}, val))
}

// Get retrieves a value of type T from the request context.
// Returns the value and true if found, or the zero value and false if not found.
//
// Example:
//
//	if userID, ok := mux.Get[string](r, "userID"); ok {
//	    // use userID
//	}
func Get[T any](r *http.Request, key string) (T, bool) {
	if r == nil {
		var zero T
		return zero, false
	}
	val := r.Context().Value(userCtxKey{name: key})
	if val == nil {
		var zero T
		return zero, false
	}
	if typed, ok := val.(T); ok {
		return typed, true
	}
	var zero T
	return zero, false
}

// Query gets a URL query parameter by key.
func Query(req *http.Request, key string) string {
	return req.URL.Query().Get(key)
}

// BindJSON decodes the JSON request body into dst.
//
// Example:
//
//	var user CreateUserRequest
//	if err := mux.BindJSON(req, &user); err != nil {
//	    return err
//	}
func BindJSON(r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return io.EOF
	}
	defer func() { _ = r.Body.Close() }()
	return json.UnmarshalRead(r.Body, dst)
}

// JSON sends a JSON response with status code and Content-Type header.
//
// Example:
//
//	return mux.JSON(w, http.StatusOK, mux.Map{"status": "ok"})
func JSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.MarshalWrite(w, data)
}

// Text sends a plain text response with the given status code.
func Text(w http.ResponseWriter, status int, value string) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write([]byte(value))
	return err
}

// Redirect redirects the request to url (default status: 302 Found).
func Redirect(w http.ResponseWriter, r *http.Request, url string, status ...int) error {
	code := http.StatusFound
	if len(status) > 0 {
		code = status[0]
	}
	http.Redirect(w, r, url, code)
	return nil
}

// NoContent sends an HTTP status code without a response body (e.g. 204).
func NoContent(w http.ResponseWriter, status int) error {
	w.WriteHeader(status)
	return nil
}
