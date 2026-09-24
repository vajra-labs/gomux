package mux

import "net/http"

// Middlewares represents a slice of middleware handlers.
type Middlewares []Middleware

// Chain converts a slice of middlewares into a Middlewares stack.
func Chain(middlewares ...Middleware) Middlewares {
	return Middlewares(middlewares)
}

// Handler wraps an endpoint with the middleware chain.
func (mws Middlewares) Handler(endpoint http.Handler) http.Handler {
	return chain(mws, endpoint)
}

// HandlerFunc wraps an http.HandlerFunc with the middleware chain.
func (mws Middlewares) HandlerFunc(endpoint http.HandlerFunc) http.HandlerFunc {
	return chain(mws, endpoint).ServeHTTP
}

// chain composes middlewares in standard FIFO execution order.
func chain(middlewares []Middleware, endpoint http.Handler) http.Handler {
	if len(middlewares) == 0 {
		return endpoint
	}
	h := middlewares[len(middlewares)-1](endpoint)
	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
