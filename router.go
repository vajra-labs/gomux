package mux

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Mux wraps standard http.ServeMux with route groups, middlewares, and error handling.
type Mux struct {
	mux          *http.ServeMux
	middlewares  Middlewares
	prefix       string
	errHandler   ErrorHandler
	notFound     http.HandlerFunc
	root         *Mux
	routesLocked bool
	rootCount    int
	compileOnce  *sync.Once
	compiled     *http.Handler
}

// New creates a new Router instance.
func New() *Mux {
	var compiled http.Handler
	return &Mux{
		mux:         http.NewServeMux(),
		errHandler:  defaultErrHandler,
		compileOnce: new(sync.Once),
		compiled:    &compiled,
	}
}

// OnError sets a centralized error handler for handlers returning an error.
func (m *Mux) OnError(handler ErrorHandler) { m.errHandler = handler }

// NotFound sets a custom handler for 404 (Not Found) requests.
func (m *Mux) NotFound[H HandlerType](handler H) {
	root := m
	if m.root != nil {
		root = m.root
	}
	root.notFound = m.toHandlerFunc(handler)
}

// handleError executes the error handler when a route returns an error.
func (m *Mux) handleError(w http.ResponseWriter, req *http.Request, err error) {
	handler := m.errHandler
	if handler == nil {
		handler = defaultErrHandler
	}
	if handlerErr := handler(w, req, err); handlerErr != nil {
		http.Error(w, handlerErr.Error(), http.StatusInternalServerError)
	}
}

// toHandlerFunc adapts standard func(w, r) or error-returning func(w, r) error into http.HandlerFunc.
func (m *Mux) toHandlerFunc(handler any) http.HandlerFunc {
	switch fn := handler.(type) {
	case http.HandlerFunc:
		return fn
	case func(http.ResponseWriter, *http.Request):
		return fn
	case Handler:
		return func(w http.ResponseWriter, req *http.Request) {
			if err := fn(w, req); err != nil {
				m.handleError(w, req, err)
			}
		}
	case func(http.ResponseWriter, *http.Request) error:
		return func(w http.ResponseWriter, req *http.Request) {
			if err := fn(w, req); err != nil {
				m.handleError(w, req, err)
			}
		}
	case http.Handler:
		return fn.ServeHTTP
	default:
		panic(fmt.Sprintf("mux: unsupported handler signature %T", handler))
	}
}

// Use adds middlewares to the router. Must be called before registering routes.
func (m *Mux) Use(middlewares ...Middleware) {
	if m.routesLocked {
		panic("mux: Use called after routes were registered; add middlewares before registering routes or use With for scoped middleware")
	}
	m.middlewares = append(m.middlewares, middlewares...)
}

// With creates a sub-router with scoped middlewares for inline chaining or grouping.
//
// Example:
//
//	r.With(auth).Get("/profile", profileHandler)
//	admin := r.With(authMiddleware)
//	admin.Get("/dashboard", dashboardHandler)
func (m *Mux) With(middle ...Middleware) *Mux {
	mws := make(Middlewares, len(m.middlewares), len(m.middlewares)+len(middle))
	copy(mws, m.middlewares)
	mws = append(mws, middle...)
	root := m.root
	rootCount := m.rootCount
	if root == nil {
		root = m
		rootCount = len(m.middlewares)
	}
	return &Mux{
		mux:         m.mux,
		middlewares: mws,
		prefix:      m.prefix,
		errHandler:  m.errHandler,
		notFound:    m.notFound,
		root:        root,
		rootCount:   rootCount,
		compileOnce: root.compileOnce,
		compiled:    root.compiled,
	}
}

// Route mounts sub-routes under a common URL prefix.
//
// Example:
//
//	r.Route("/users", func(users *mux.Mux) {
//	    users.Get("/{id}", getUser)
//	    users.Post("/", createUser)
//	})
func (m *Mux) Route(prefix string, fn func(r *Mux)) {
	if fn == nil {
		panic(fmt.Sprintf("mux: attempting to Route() a nil handler on '%s'", prefix))
	}
	root := m.root
	rootCount := m.rootCount
	if root == nil {
		root = m
		rootCount = len(m.middlewares)
	}
	subGroup := &Mux{
		mux:         m.mux,
		middlewares: m.middlewares,
		prefix:      cleanPrefix(m.prefix, prefix),
		errHandler:  m.errHandler,
		notFound:    m.notFound,
		root:        root,
		rootCount:   rootCount,
		compileOnce: root.compileOnce,
		compiled:    root.compiled,
	}
	fn(subGroup)
}

// On registers a route handler for a custom HTTP method.
func (m *Mux) On[H HandlerType](method, path string, handler H, middle ...Middleware) {
	m.handle(strings.ToUpper(method), path, handler, middle...)
}

// Get registers a GET route handler.
func (m *Mux) Get[H HandlerType](path string, handler H, middle ...Middleware) {
	m.handle(http.MethodGet, path, handler, middle...)
}

// Post registers a POST route handler.
func (m *Mux) Post[H HandlerType](path string, handler H, middle ...Middleware) {
	m.handle(http.MethodPost, path, handler, middle...)
}

// Put registers a PUT route handler.
func (m *Mux) Put[H HandlerType](path string, handler H, middle ...Middleware) {
	m.handle(http.MethodPut, path, handler, middle...)
}

// Delete registers a DELETE route handler.
func (m *Mux) Delete[H HandlerType](path string, handler H, middle ...Middleware) {
	m.handle(http.MethodDelete, path, handler, middle...)
}

// Patch registers a PATCH route handler.
func (m *Mux) Patch[H HandlerType](path string, handler H, middle ...Middleware) {
	m.handle(http.MethodPatch, path, handler, middle...)
}

// Options registers an OPTIONS route handler.
func (m *Mux) Options[H HandlerType](path string, handler H, middle ...Middleware) {
	m.handle(http.MethodOptions, path, handler, middle...)
}

// Head registers a HEAD route handler.
func (m *Mux) Head[H HandlerType](path string, handler H, middle ...Middleware) {
	m.handle(http.MethodHead, path, handler, middle...)
}

// All registers a route that matches any HTTP method.
func (m *Mux) All[H HandlerType](path string, handler H, middle ...Middleware) {
	m.handle("ALL", path, handler, middle...)
}

// Handle registers a standard http.Handler for a method and path.
func (m *Mux) Handle(method, path string, handler http.Handler, middle ...Middleware) {
	m.lockRoot()
	fullPath := cleanPrefix(m.prefix, path)
	pattern := buildPattern(method, fullPath)
	chained := m.wrapMiddleware(handler, middle...)
	m.mux.Handle(pattern, chained)
}

// HandleFiles serves static files from a directory.
//
// Example:
//
//	r.HandleFiles("/static", http.Dir("./public"))
func (m *Mux) HandleFiles(pattern string, root http.FileSystem, middle ...Middleware) {
	m.lockRoot()
	if !strings.HasSuffix(pattern, "/") {
		pattern += "/"
	}
	fullPath := cleanPrefix(m.prefix, pattern)
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	var handler http.Handler
	if fullPath == "/" {
		handler = http.FileServer(root)
	} else {
		handler = http.StripPrefix(strings.TrimSuffix(fullPath, "/"), http.FileServer(root))
	}
	chained := m.wrapMiddleware(handler, middle...)
	m.mux.Handle(fullPath, chained)
}

// Mount attaches an external http.Handler under a URL prefix.
//
// Example:
//
//	r.Mount("/swagger", httpSwagger.Handler())
func (m *Mux) Mount(pattern string, handler http.Handler, middle ...Middleware) {
	if handler == nil {
		panic(fmt.Sprintf("mux: attempting to Mount() a nil handler on '%s'", pattern))
	}
	m.lockRoot()
	fullPath := cleanPrefix(m.prefix, pattern)
	fullPath = strings.TrimSuffix(fullPath, "/")
	mountPattern := fullPath + "/"
	if fullPath == "" {
		mountPattern = "/"
	}
	chained := m.wrapMiddleware(http.StripPrefix(fullPath, handler), middle...)
	m.mux.Handle(mountPattern, chained)
}

// ServeHTTP implements http.Handler.
func (m *Mux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	root := m
	if m.root != nil {
		root = m.root
	}
	// Fast-path: When no custom 404 and no global middlewares,
	// dispatch directly to ServeMux with ZERO overhead!
	if root.notFound == nil && len(root.middlewares) == 0 {
		m.mux.ServeHTTP(w, req)
		return
	}
	root.compileOnce.Do(root.compile)
	(*root.compiled).ServeHTTP(w, req)
}

// compile prepares the root middleware and 404 pipeline once at startup.
func (m *Mux) compile() {
	var handler http.Handler = m.mux
	if m.notFound != nil {
		notFoundHandler := m.notFound
		handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			h, pattern := m.mux.Handler(req)
			if pattern != "" {
				h.ServeHTTP(w, req)
				return
			}
			notFoundHandler.ServeHTTP(w, req)
		})
	}
	// Wrap root-level global middlewares once at startup using the middleware chain.
	if len(m.middlewares) > 0 {
		inner := m.middlewares.Handler(handler)
		handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Pattern == "" {
				if _, pattern := m.mux.Handler(req); pattern != "" {
					req.Pattern = pattern
				}
			}
			inner.ServeHTTP(w, req)
		})
	}
	*m.compiled = handler
}

// ServeMux returns the underlying *http.ServeMux instance.
func (m *Mux) ServeMux() *http.ServeMux { return m.mux }

// lockRoot marks routes as registered so no root-level middlewares can be added afterwards.
func (m *Mux) lockRoot() {
	m.routesLocked = true
	if m.root != nil {
		m.root.routesLocked = true
	}
}

// wrapMiddleware applies group-level and route-level (inline) middlewares to the handler.
func (m *Mux) wrapMiddleware(handler http.Handler, middle ...Middleware) http.Handler {
	if len(middle) > 0 {
		handler = Middlewares(middle).Handler(handler)
	}
	if m.root == nil {
		return handler
	}
	start := min(m.rootCount, len(m.middlewares))
	return Middlewares(m.middlewares[start:]).Handler(handler)
}

// handle registers the route pattern with ServeMux and applies group-level and inline middlewares.
func (m *Mux) handle[H HandlerType](method, path string, handler H, middle ...Middleware) {
	m.lockRoot()
	fullPath := cleanPrefix(m.prefix, path)
	pattern := buildPattern(method, fullPath)
	handlerFunc := m.toHandlerFunc(handler)
	chainedHandler := m.wrapMiddleware(handlerFunc, middle...)
	m.mux.Handle(pattern, chainedHandler)
}

// buildPattern formats the Go 1.22+ ServeMux routing pattern.
func buildPattern(method, fullPath string) string {
	fullPath = strings.TrimSpace(fullPath)
	if fullPath == "/" {
		fullPath = "/{$}"
	}
	if method == "" || method == "ALL" {
		return fullPath
	}
	return strings.ToUpper(method) + " " + fullPath
}

// cleanPrefix merges a base prefix and a sub-path pattern, ensuring valid formatting.
func cleanPrefix(base, sub string) string {
	base = strings.TrimSpace(base)
	sub = strings.TrimSpace(sub)
	if base == "" || base == "/" {
		if sub == "" || sub == "/" {
			return "/"
		}
		if !strings.HasPrefix(sub, "/") {
			return "/" + sub
		}
		return sub
	}
	base = strings.TrimSuffix(base, "/")
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	if sub == "" || sub == "/" {
		return base
	}
	if !strings.HasPrefix(sub, "/") {
		sub = "/" + sub
	}
	return base + sub
}
