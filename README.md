# routemux

A lightweight, high-performance, and ergonomic HTTP micro-router built directly on Go 1.27+ standard library `http.ServeMux`.

`routemux` combines the speed and stability of the Go standard library with developer-friendly ergonomics inspired by modern frameworks like Chi and Fiber—**without external dependencies, without regex overhead, and with 100% compile-time type safety**.

---

## Highlights

- 🚀 **Modern Go 1.27+ Architecture:** Leverages next-generation standard library capabilities including `encoding/json/v2` and `errors.AsType`.
- ⚡ **Ultra-Fast & Lightweight:** Sub-microsecond routing (~170ns) and ~1.3 KB RAM footprint per route.
- 🎯 **Native Path Patterns:** Uses standard library path patterns (`/{id}`, `/{path...}`) and HTTP method matching.
- 🛡️ **Zero External Dependencies:** Built entirely with standard library Go packages.
- 🔒 **100% Type-Safe:** No `any` casting on routes; handlers accept both standard `func(w, r)` and ergonomic `func(w, r) error`.
- 🧅 **Two-Tier Middleware Architecture:**
  - **Global (`Use`):** Applies to all routes and custom 404 handlers.
  - **Scoped (`With`):** Applies only to specific routes or chained sub-groups without leaking.
- 🌳 **Modular Routing (`Route`):** Clean sub-routing closures for feature modules.
- 📦 **Built-in Helpers:** JSON serialization (`encoding/json/v2`), request binding, query helpers, typed context helpers (`Set`/`Get`), and structured HTTP errors (`errorx`).

---

## Requirements

- **Go 1.27+** is required (utilizes `encoding/json/v2` and `errors.AsType`).

---

## Installation

```bash
go get github.com/vajra-labs/routemux
```

---

## Quick Start

```go
package main

import (
	"log"
	"net/http"

	"github.com/vajra-labs/routemux"
)

func main() {
	r := routemux.New()

	// 1. Global Middleware (Logger)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			log.Printf("[%s] %s", req.Method, req.URL.Path)
			next.ServeHTTP(w, req)
		})
	})

	// 2. Standard & Error-Returning Handlers
	r.Get("/", func(w http.ResponseWriter, req *http.Request) error {
		return routemux.JSON(w, http.StatusOK, routemux.Map{
			"message": "Welcome to routemux!",
			"status":  "ok",
		})
	})

	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
		userID := req.PathValue("id")
		return routemux.JSON(w, http.StatusOK, routemux.Map{
			"user_id": userID,
		})
	})

	log.Println("Server listening on :8080")
	http.ListenAndServe(":8080", r)
}
```

---

## Routing Guide

### HTTP Methods

All standard HTTP verbs are supported:

```go
r.Get("/items", listItems)
r.Post("/items", createItem)
r.Put("/items/{id}", updateItem)
r.Patch("/items/{id}", patchItem)
r.Delete("/items/{id}", deleteItem)
r.Options("/items", optionsHandler)
r.Head("/items/{id}", headHandler)

// Match any HTTP method
r.All("/health", healthHandler)

// Custom HTTP method
r.On("PURGE", "/cache", purgeHandler)
```

### Path Parameters

`routemux` leverages Go 1.22+ native path pattern matching:

```go
// Single Path Parameter
r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
    id := req.PathValue("id")
    return routemux.Text(w, http.StatusOK, "User ID: "+id)
})

// Multiple Path Parameters
r.Get("/orgs/{orgId}/repos/{repoId}", func(w http.ResponseWriter, req *http.Request) error {
    org := req.PathValue("orgId")
    repo := req.PathValue("repoId")
    return routemux.JSON(w, http.StatusOK, routemux.Map{"org": org, "repo": repo})
})

// Wildcard / Catch-All
r.Get("/static/{file...}", func(w http.ResponseWriter, req *http.Request) error {
    filePath := req.PathValue("file")
    return routemux.Text(w, http.StatusOK, "Path: "+filePath)
})
```

---

## Middlewares

### 1. Global Middlewares (`Use`)

Middlewares registered via `r.Use()` wrap the entire router, executing on every matching route and even on **404 Not Found** responses:

```go
r.Use(loggerMiddleware)
r.Use(corsMiddleware)
```

> **Note:** `Use()` must be declared before defining routes. Registering `Use()` after routes will safely panic to avoid silent bugs.

### 2. Scoped & Inline Middlewares (`With`)

Use `With()` to apply middlewares to a single route or a chain without leaking to other endpoints:

```go
// Single Route with Middleware
r.With(authGuard).Get("/me", getProfile)

// Route with Multiple Middlewares (auth -> rateLimit -> handler)
r.With(authGuard, rateLimit).Post("/transfer", transferFunds)

// Sub-router grouping
admin := r.With(authGuard, adminOnly)
admin.Get("/dashboard", adminDashboard)
admin.Delete("/users/{id}", deleteUser)
```

---

## Sub-Routing & Modularity (`Route`)

Group related routes under a common URL prefix cleanly using closures:

```go
r.Route("/api/v1", func(api *routemux.Mux) {
    api.Use(apiVersionLogger)

    // Mount /api/v1/auth
    api.Route("/auth", func(auth *routemux.Mux) {
        auth.Post("/login", loginHandler)
        auth.Post("/register", registerHandler)
        auth.With(authGuard).Post("/logout", logoutHandler)
    })

    // Mount /api/v1/users
    api.Route("/users", func(users *routemux.Mux) {
        users.Use(authGuard)
        users.Get("/", listUsers)
        users.Get("/{id}", getUser)
    })
})
```

### Modular Router Pattern (Production Structure)

For large projects, organize route registration by domain:

```go
type UserRouter struct {
    guard   *Guard
    handler *UserHandler
}

func (u *UserRouter) Register(app *routemux.Mux) {
    app.Route("/users", func(r *routemux.Mux) {
        r.Use(u.guard.Auth)
        r.Get("/", u.handler.List)
        r.Post("/", u.handler.Create)
        r.Get("/{id}", u.handler.Get)
    })
}

// In main.go:
userRouter.Register(app)
postRouter.Register(app)
```

---

## Error Handling

### 1. Returning Errors from Handlers

Handlers can return standard Go `error` values or structured `errorx.HttpError`:

```go
r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
    user, err := findUser(req.PathValue("id"))
    if err != nil {
        return routemux.NotFoundError("User not found", "USER_NOT_FOUND", routemux.WithCause(err))
    }
    return routemux.JSON(w, http.StatusOK, user)
})
```

### 2. Custom Error Handler (`OnError`)

Configure how errors are formatted and returned to the client:

```go
r.OnError(func(w http.ResponseWriter, req *http.Request, err error) error {
    if httpErr, ok := routemux.IsHttpError(err); ok {
        return httpErr.ToJSON(w)
    }
    // Fallback internal server error
    return routemux.JSON(w, http.StatusInternalServerError, routemux.Map{
        "error": "Internal Server Error",
    })
})
```

### 3. Structured `errorx` Constructors

Available structured error helpers `(message, code, opts...)`:

- `routemux.BadRequestError(msg, code, opts...)` (400)
- `routemux.UnauthorizedError(msg, code, opts...)` (401)
- `routemux.ForbiddenError(msg, code, opts...)` (403)
- `routemux.NotFoundError(msg, code, opts...)` (404)
- `routemux.ConflictError(msg, code, opts...)` (409)
- `routemux.InternalServerError(msg, code, opts...)` (500)
- `routemux.NewHttpError(status, msg, opts...)` (Custom Status)

---

## Custom 404 Handler (`NotFound`)

Set a custom JSON or HTML 404 response handler. Global middlewares (`Use`) will still execute on 404 requests (e.g., logging request paths and response statuses):

```go
r.NotFound(func(w http.ResponseWriter, req *http.Request) error {
    return routemux.JSON(w, http.StatusNotFound, routemux.Map{
        "error":  "Route not found",
        "path":   req.URL.Path,
        "method": req.Method,
    })
})
```

---

## Helper Utilities

| Helper                                 | Description                                                                     | Example                                               |
| :------------------------------------- | :------------------------------------------------------------------------------ | :---------------------------------------------------- |
| `routemux.Set(req, key, val)`          | Store a key-value pair in the request context (returns updated `*http.Request`) | `req = routemux.Set(req, "userID", "123")`            |
| `routemux.Get[T](req, key)`            | Retrieve a typed value `T` from the request context (returns `(T, bool)`)       | `userID, ok := routemux.Get[string](req, "userID")`   |
| `routemux.JSON(w, code, data)`         | Serialize and write JSON response with `Content-Type: application/json`         | `routemux.JSON(w, 200, routemux.Map{"status": "ok"})` |
| `routemux.BindJSON(req, &dest)`        | Decode JSON request body into a struct or map                                   | `err := routemux.BindJSON(req, &user)`                |
| `routemux.Text(w, code, text)`         | Write plain text response                                                       | `routemux.Text(w, 200, "hello")`                      |
| `routemux.Query(req, key)`             | Get query parameter with whitespace trimmed                                     | `page := routemux.Query(req, "page")`                 |
| `routemux.Redirect(w, req, url, code)` | Redirect client (defaults to 302 Found)                                         | `routemux.Redirect(w, req, "/login")`                 |
| `routemux.NoContent(w)`                | Return 204 No Content                                                           | `routemux.NoContent(w)`                               |

### Context Values Example:

```go
// In Auth Middleware:
authMiddleware := func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        req = routemux.Set(req, "userID", "user_42")
        next.ServeHTTP(w, req)
    })
}

// In Route Handler:
r.With(authMiddleware).Get("/profile", func(w http.ResponseWriter, req *http.Request) error {
    userID, ok := routemux.Get[string](req, "userID")
    if !ok {
        return routemux.UnauthorizedError("Unauthorized")
    }
    return routemux.JSON(w, http.StatusOK, routemux.Map{"user_id": userID})
})
```

---

## Static Files & Mounting External Handlers

### Static Files (`HandleFiles`)

Serve files from `http.FileSystem` (e.g., disk or `embed.FS`):

```go
r.HandleFiles("/static", http.Dir("./public"))
```

### Mount External Handlers (`Mount`)

Mount third-party routers, Swagger UI, or `http.Handler` packages:

```go
r.Mount("/swagger", swaggerHandler)
r.Mount("/debug/pprof", pprofHandler)
```

---

## Performance & Benchmarks

Tested on Apple M4 (Go 1.27 darwin/arm64) using `b.Loop()`:

```text
================ ROUTER RAM FOOTPRINT ================
Routes: 100    | RAM Consumed: 135.35 KB  | Per-Route: ~1.3 KB
Routes: 1,000  | RAM Consumed: 1.25 MB    | Per-Route: ~1.3 KB
Routes: 10,000 | RAM Consumed: 12.32 MB   | Per-Route: ~1.2 KB
======================================================

BenchmarkMux_StaticRoute-10           15,676,762 ops    76.08 ns/op     0 B/op   0 allocs/op
BenchmarkMux_SingleParamRoute-10      17,476,978 ops    65.24 ns/op    16 B/op   1 allocs/op
BenchmarkMux_MultiParamRoute-10       10,084,520 ops   116.20 ns/op    48 B/op   2 allocs/op
BenchmarkMux_WithMiddlewarePipeline-10 9,486,247 ops   122.70 ns/op    32 B/op   2 allocs/op
BenchmarkMux_JSONResponse-10           2,399,328 ops   499.30 ns/op   104 B/op   8 allocs/op
```

- **Routing Speed:** ~65–122 nanoseconds per request (~10M–17M req/sec single-core).
- **RAM Efficiency:** ~12.6 MB for 10,000 registered routes (~1.3 KB per route).
- **Zero Allocations on Static Routes:** Static routes run on pure fast-path with **0 B/op and 0 allocs/op**.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
