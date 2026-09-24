# gomux

[![Go Reference](https://pkg.go.dev/badge/github.com/vajra-labs/gomux.svg)](https://pkg.go.dev/github.com/vajra-labs/gomux)
[![Go Report Card](https://goreportcard.com/badge/github.com/vajra-labs/gomux)](https://goreportcard.com/report/github.com/vajra-labs/gomux)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.27-00ADD8?logo=go)](go.mod)

`gomux` is an idiomatic, lightweight, and high-performance HTTP micro-router and middleware stack for Go built directly on Go 1.27+ standard library `http.ServeMux`.

If you love the reliability and speed of the Go standard library, but want the ergonomics of modern web frameworks like Chi or Fiber—without adding heavy external dependencies, regex overhead, or non-idiomatic abstractions—then `gomux` is a great fit.

---

## Highlights

- 🚀 **Built for Modern Go (1.27+):** Leverages next-generation standard library capabilities including `encoding/json/v2` and `errors.AsType`.
- ⚡ **Zero-Overhead Routing:** Sub-microsecond execution (~75ns static routes) and zero heap allocations (`0 B/op`, `0 allocs/op`).
- 🛡️ **Zero External Dependencies:** Built 100% on the Go standard library.
- 🎯 **Native Path Patterns:** Uses Go's native pattern matching (`/{id}`, wildcards `/{file...}`) and HTTP verb routing.
- 🔒 **100% Compile-Time Type Safety:** No generic `any` casting on routes; accepts both `func(w, r)` and ergonomic `func(w, r) error`.
- 🧅 **Three-Tier Middleware Architecture:**
  - **Global (`Use`):** Covers all routes and custom 404 handlers.
  - **Scoped Groups (`Route` & `With`):** Applies isolated middlewares to specific sub-groups without leaking.
  - **Route-Level Inline:** Directly attach middlewares to any endpoint (`r.Get("/path", handler, auth, rateLimit)`) with zero allocation overhead.
- 🌳 **Sub-Routing & Route Groups (`Route`):** Clean, modular sub-routing closures for feature modules.
- 📦 **Built-in Helpers:** JSON serialization (`encoding/json/v2`), request binding, query helpers, typed context storage (`Set`/`Get`), and structured HTTP errors (`errorx`).

---

## Getting Started

After installing Go (>= 1.27), create your first `.go` file. We'll call it `server.go`:

```go
package main

import (
	"log"
	"net/http"

	"github.com/vajra-labs/gomux"
)

func main() {
	r := gomux.New()

	// 1. Global Middleware (Logger)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			log.Printf("[%s] %s", req.Method, req.URL.Path)
			next.ServeHTTP(w, req)
		})
	})

	// 2. Handlers with error-return ergonomics
	r.Get("/", func(w http.ResponseWriter, req *http.Request) error {
		return gomux.JSON(w, http.StatusOK, gomux.Map{
			"message": "Welcome to gomux!",
			"status":  "ok",
		})
	})

	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
		userID := req.PathValue("id")
		return gomux.JSON(w, http.StatusOK, gomux.Map{
			"user_id": userID,
		})
	})

	log.Println("Server listening on :8080")
	http.ListenAndServe(":8080", r)
}
```

Install the package:

```bash
go get github.com/vajra-labs/gomux
```

Then run your server:

```bash
go run server.go
```

You now have a production-grade, standard `net/http` web server running on `localhost:8080`.

---

## Routing

### HTTP Methods

All standard HTTP methods are directly supported with dedicated methods:

```go
r.Get("/items", listItems)
r.Post("/items", createItem)
r.Put("/items/{id}", updateItem)
r.Patch("/items/{id}", patchItem)
r.Delete("/items/{id}", deleteItem)
r.Options("/items", optionsHandler)
r.Head("/items/{id}", headHandler)

// Match any HTTP method on a path
r.All("/health", healthHandler)

// Register custom HTTP methods
r.On("PURGE", "/cache", purgeHandler)
```

### Path Parameters

`gomux` leverages Go's native path pattern matching:

```go
// Single Path Parameter
r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
    id := req.PathValue("id")
    return gomux.Text(w, http.StatusOK, "User ID: "+id)
})

// Multiple Path Parameters
r.Get("/orgs/{orgId}/repos/{repoId}", func(w http.ResponseWriter, req *http.Request) error {
    org := req.PathValue("orgId")
    repo := req.PathValue("repoId")
    return gomux.JSON(w, http.StatusOK, gomux.Map{"org": org, "repo": repo})
})

// Wildcard / Catch-All
r.Get("/static/{file...}", func(w http.ResponseWriter, req *http.Request) error {
    filePath := req.PathValue("file")
    return gomux.Text(w, http.StatusOK, "File: "+filePath)
})
```

### Flexible Handlers

Handlers accept either standard `http.HandlerFunc` or error-returning functions:

```go
// 1. Standard signature:
r.Get("/ping", func(w http.ResponseWriter, req *http.Request) {
    w.Write([]byte("pong"))
})

// 2. Error-returning signature:
r.Get("/ping", func(w http.ResponseWriter, req *http.Request) error {
    return gomux.JSON(w, http.StatusOK, gomux.Map{"message": "pong"})
})
```

---

## Middlewares

Middleware in `gomux` follows the standard Go signature:

```go
type Middleware func(http.Handler) http.Handler
```

### `Use()` (Global Middleware)

Middlewares registered via `r.Use()` wrap the entire router, executing on every matching route and even on **404 Not Found** responses:

```go
r.Use(loggerMiddleware)
r.Use(corsMiddleware)
```

> **Note:** `Use()` must be declared before registering routes. Registering `Use()` after routes will safely panic to prevent order-of-execution bugs.

### Route-Level Inline Middleware

Attach middlewares directly to individual route registrations in standard FIFO execution order:

```go
// Single route with inline middleware
r.Get("/profile", getProfile, authGuard)

// Multiple inline middlewares chained in FIFO order (authGuard -> rateLimit -> handler)
r.Post("/transfer", transferFunds, authGuard, rateLimit)

// Custom HTTP method with inline middleware
r.On("PURGE", "/cache", purgeCache, adminOnly)

// Standard http.Handler with inline middleware
r.Handle("GET", "/metrics", promHandler, basicAuth)

// Static file serving with inline middleware
r.HandleFiles("/static", http.Dir("./public"), cacheControl)
```

### `With()` (Scoped Sub-Router Grouping)

`With()` creates an isolated sub-router with shared scoped middlewares for clean route grouping without leaking to sibling endpoints:

```go
// Sub-router grouping with shared middleware
admin := r.With(authGuard, adminOnly)
admin.Get("/dashboard", adminDashboard)
admin.Delete("/users/{id}", deleteUser)
```

---

## Sub-Routing (`Route`)

Group related routes cleanly under a common URL prefix using closures:

```go
r.Route("/api/v1", func(api *gomux.Mux) {
    api.Use(apiVersionLogger)

    // Sub-route: /api/v1/auth
    api.Route("/auth", func(auth *gomux.Mux) {
        auth.Post("/login", loginHandler)
        auth.Post("/register", registerHandler)
        auth.With(authGuard).Post("/logout", logoutHandler)
    })

    // Sub-route: /api/v1/users
    api.Route("/users", func(users *gomux.Mux) {
        users.Use(authGuard)
        users.Get("/", listUsers)
        users.Get("/{id}", getUser)
    })
})
```

### Modular Router Pattern (Production Organization)

For large codebases, organize routes by domain controller:

```go
type UserRouter struct {
    guard   *Guard
    handler *UserHandler
}

func (u *UserRouter) Register(app *gomux.Mux) {
    app.Route("/users", func(r *gomux.Mux) {
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

Handlers can return standard Go `error` values or structured `HttpError`:

```go
r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
    user, err := findUser(req.PathValue("id"))
    if err != nil {
        return gomux.NotFoundError("User not found", "USER_NOT_FOUND", gomux.WithCause(err))
    }
    return gomux.JSON(w, http.StatusOK, user)
})
```

### 2. Centralized Error Handler (`OnError`)

Configure how errors are formatted and returned to clients globally:

```go
r.OnError(func(w http.ResponseWriter, req *http.Request, err error) error {
    if httpErr, ok := gomux.IsHttpError(err); ok {
        return httpErr.ToJSON(w)
    }
    // Fallback internal server error
    return gomux.JSON(w, http.StatusInternalServerError, gomux.Map{
        "error": "Internal Server Error",
    })
})
```

### 3. Structured Error Constructors

Pre-built structured error helpers `(message, code, opts...)`:

- `gomux.BadRequestError(msg, code, opts...)` (400)
- `gomux.UnauthorizedError(msg, code, opts...)` (401)
- `gomux.ForbiddenError(msg, code, opts...)` (403)
- `gomux.NotFoundError(msg, code, opts...)` (404)
- `gomux.ConflictError(msg, code, opts...)` (409)
- `gomux.InternalServerError(msg, code, opts...)` (500)
- `gomux.NewHttpError(status, msg, opts...)` (Custom Status)

---

## Custom 404 Handler (`NotFound`)

Set a custom JSON or HTML 404 response handler. Global middlewares (`Use`) continue to execute on 404 requests (e.g. logging and metrics):

```go
r.NotFound(func(w http.ResponseWriter, req *http.Request) error {
    return gomux.JSON(w, http.StatusNotFound, gomux.Map{
        "error":  "Route not found",
        "path":   req.URL.Path,
        "method": req.Method,
    })
})
```

---

## Built-in Helpers

| Helper                              | Description                                                         | Example                                          |
| :---------------------------------- | :------------------------------------------------------------------ | :----------------------------------------------- |
| `gomux.Set(req, key, val)`          | Stores a value in request context (returns updated `*http.Request`) | `req = gomux.Set(req, "userID", "123")`          |
| `gomux.Get[T](req, key)`            | Retrieves typed value `T` from context (returns `(T, bool)`)        | `userID, ok := gomux.Get[string](req, "userID")` |
| `gomux.JSON(w, code, data)`         | Serializes and writes JSON using Go's `encoding/json/v2`            | `gomux.JSON(w, 200, gomux.Map{"status": "ok"})`  |
| `gomux.BindJSON(req, &dest)`        | Decodes JSON request body into a struct or map                      | `err := gomux.BindJSON(req, &user)`              |
| `gomux.Text(w, code, text)`         | Writes plain text response                                          | `gomux.Text(w, 200, "hello")`                    |
| `gomux.Query(req, key)`             | Retrieves query parameter with whitespace trimmed                   | `page := gomux.Query(req, "page")`               |
| `gomux.Redirect(w, req, url, code)` | Redirects client (defaults to 302 Found)                            | `gomux.Redirect(w, req, "/login")`               |
| `gomux.NoContent(w)`                | Sends HTTP 204 No Content                                           | `gomux.NoContent(w)`                             |

---

## Static Files & Mounting External Handlers

### Static Files (`HandleFiles`)

Serve static files directly from `http.FileSystem` (e.g., disk or `embed.FS`):

```go
r.HandleFiles("/static", http.Dir("./public"))
```

### Mount External Handlers (`Mount`)

Mount sub-routers, Swagger UI, or third-party handlers under a prefix:

```go
r.Mount("/swagger", swaggerHandler)
r.Mount("/debug/pprof", pprofHandler)
```

---

## Third-Party Middleware

Because `gomux` adheres strictly to standard Go `func(http.Handler) http.Handler`, it is **100% compatible** with standard `net/http` middlewares across the Go ecosystem.

### Compatible Middleware List

| Middleware                                                                   | Author                                               | Description                                                                                                                     |
| :--------------------------------------------------------------------------- | :--------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------ |
| [authz](https://github.com/casbin/negroni-authz)                             | [Yang Luo](https://github.com/hsluoyz)               | ACL, RBAC, ABAC Authorization middleware based on [Casbin](https://github.com/casbin/casbin)                                    |
| [binding](https://github.com/mholt/binding)                                  | [Matt Holt](https://github.com/mholt)                | Data binding from HTTP requests into structs                                                                                    |
| [cloudwatch](https://github.com/cvillecsteele/negroni-cloudwatch)            | [Colin Steele](https://github.com/cvillecsteele)     | AWS cloudwatch metrics middleware                                                                                               |
| [cors](https://github.com/rs/cors)                                           | [Olivier Poitrey](https://github.com/rs)             | [Cross Origin Resource Sharing](http://www.w3.org/TR/cors/) (CORS) support                                                      |
| [csp](https://github.com/awakenetworks/csp)                                  | [Awake Networks](https://github.com/awakenetworks)   | [Content Security Policy](https://www.w3.org/TR/CSP2/) (CSP) support                                                            |
| [delay](https://github.com/jeffbmartinez/delay)                              | [Jeff Martinez](https://github.com/jeffbmartinez)    | Add delays/latency to endpoints. Useful when testing effects of high latency                                                    |
| [New Relic Go Agent](https://github.com/yadvendar/negroni-newrelic-go-agent) | [Yadvendar Champawat](https://github.com/yadvendar)  | Official [New Relic Go Agent](https://github.com/newrelic/go-agent)                                                             |
| [gorelic](https://github.com/jingweno/negroni-gorelic)                       | [Jingwen Owen Ou](https://github.com/jingweno)       | New Relic agent for Go runtime                                                                                                  |
| [Graceful](https://github.com/tylerb/graceful)                               | [Tyler Bunnell](https://github.com/tylerb)           | Graceful HTTP Shutdown                                                                                                          |
| [gzip](https://github.com/phyber/negroni-gzip)                               | [phyber](https://github.com/phyber)                  | GZIP response compression                                                                                                       |
| [JWT Middleware](https://github.com/auth0/go-jwt-middleware)                 | [Auth0](https://github.com/auth0)                    | Middleware checks for a JWT on the `Authorization` header on incoming requests and decodes it                                   |
| [JWT Middleware](https://github.com/mfuentesg/go-jwtmiddleware)              | [Marcelo Fuentes](https://github.com/mfuentesg)      | JWT middleware for golang                                                                                                       |
| [logrus](https://github.com/meatballhat/negroni-logrus)                      | [Dan Buch](https://github.com/meatballhat)           | Logrus-based logger                                                                                                             |
| [oauth2](https://github.com/goincremental/negroni-oauth2)                    | [David Bochenski](https://github.com/bochenski)      | oAuth2 middleware                                                                                                               |
| [onthefly](https://github.com/xyproto/onthefly)                              | [Alexander Rødseth](https://github.com/xyproto)      | Generate TinySVG, HTML and CSS on the fly                                                                                       |
| [permissions2](https://github.com/xyproto/permissions2)                      | [Alexander Rødseth](https://github.com/xyproto)      | Cookies, users and permissions                                                                                                  |
| [prometheus](https://github.com/zbindenren/negroni-prometheus)               | [Rene Zbinden](https://github.com/zbindenren)        | Easily create metrics endpoint for the [prometheus](http://prometheus.io) instrumentation tool                                  |
| [prometheus](https://github.com/slok/go-prometheus-middleware)               | [Xabier Larrakoetxea](https://github.com/slok)       | [Prometheus](http://prometheus.io) metrics with multiple options that follow standards                                          |
| [render](https://github.com/unrolled/render)                                 | [Cory Jacobsen](https://github.com/unrolled)         | Render JSON, XML and HTML templates                                                                                             |
| [RestGate](https://github.com/pjebs/restgate)                                | [Prasanga Siripala](https://github.com/pjebs)        | Secure authentication for REST API endpoints                                                                                    |
| [secure](https://github.com/unrolled/secure)                                 | [Cory Jacobsen](https://github.com/unrolled)         | Middleware that implements a few quick security wins                                                                            |
| [sessions](https://github.com/goincremental/negroni-sessions)                | [David Bochenski](https://github.com/bochenski)      | Session Management                                                                                                              |
| [stats](https://github.com/thoas/stats)                                      | [Florent Messa](https://github.com/thoas)            | Store information about your web application (response time, etc.)                                                              |
| [VanGoH](https://github.com/auroratechnologies/vangoh)                       | [Taylor Wrobel](https://github.com/twrobel3)         | Configurable [AWS-Style](http://docs.aws.amazon.com/AmazonS3/latest/dev/RESTAuthentication.html) HMAC authentication middleware |
| [xrequestid](https://github.com/pilu/xrequestid)                             | [Andrea Franz](https://github.com/pilu)              | Middleware that assigns a random X-Request-Id header to each request                                                            |
| [mgo session](https://github.com/joeljames/nigroni-mgo-session)              | [Joel James](https://github.com/joeljames)           | Middleware that handles creating and closing mgo sessions per request                                                           |
| [digits](https://github.com/bamarni/digits)                                  | [Bilal Amarni](https://github.com/bamarni)           | Middleware that handles [Twitter Digits](https://get.digits.com/) authentication                                                |
| [stats](https://github.com/guptachirag/stats)                                | [Chirag Gupta](https://github.com/guptachirag/stats) | Middleware that manages qps and latency stats for your endpoints and flushes to influx db                                       |
| [Chaos](https://github.com/falzm/chaos)                                      | [Marc Falzon](https://github.com/falzm)              | Middleware for injecting chaotic behavior into application in a programmatic way                                                |

### Integration Example (CORS & Security)

```go
package main

import (
    "net/http"

    "github.com/rs/cors"
    "github.com/unrolled/secure"
    "github.com/vajra-labs/gomux"
)

func main() {
    r := gomux.New()

    // 1. Cross-Origin Resource Sharing (CORS)
    c := cors.New(cors.Options{
        AllowedOrigins:   []string{"https://example.com"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
        AllowCredentials: true,
    })
    r.Use(c.Handler)

    // 2. Security Headers (HSTS, CSP, etc.)
    sec := secure.New(secure.Options{
        SSLRedirect:          true,
        STSSeconds:           31536000,
        FrameDeny:            true,
        ContentTypeNosniff:   true,
        BrowserXssFilter:     true,
    })
    r.Use(sec.Handler)

    r.Get("/api/data", func(w http.ResponseWriter, req *http.Request) error {
        return gomux.JSON(w, http.StatusOK, gomux.Map{"secure": true})
    })

    http.ListenAndServe(":8080", r)
}
```

---

## Performance & Benchmarks

Tested on Apple M4 (Go 1.27 darwin/arm64) using `b.Loop()`:

```text
================ ROUTER RAM FOOTPRINT ================
Routes: 100    | RAM Consumed: 136.68 KB  | Per-Route: ~1.3 KB
Routes: 1,000  | RAM Consumed: 1.27 MB    | Per-Route: ~1.3 KB
Routes: 5,000  | RAM Consumed: 6.21 MB    | Per-Route: ~1.2 KB
Routes: 10,000 | RAM Consumed: 12.47 MB   | Per-Route: ~1.2 KB
======================================================

BenchmarkMux_StaticRoute-10           15,960,913 ops    75.77 ns/op     0 B/op   0 allocs/op
BenchmarkMux_SingleParamRoute-10      17,752,387 ops    66.62 ns/op    16 B/op   1 allocs/op
BenchmarkMux_InlineMiddleware-10      12,347,790 ops    95.84 ns/op    16 B/op   1 allocs/op
BenchmarkMux_MultiParamRoute-10       10,028,982 ops   119.00 ns/op    48 B/op   2 allocs/op
BenchmarkMux_WithMiddlewarePipeline-10 9,626,169 ops   124.20 ns/op    32 B/op   2 allocs/op
BenchmarkMux_JSONResponse-10           2,323,332 ops   516.00 ns/op   104 B/op   8 allocs/op
```

- **Routing Speed:** ~66–124 nanoseconds per request (~10M–17.7M req/sec single-core).
- **RAM Efficiency:** ~12.5 MB for 10,000 registered routes (~1.2 KB per route).
- **Zero Allocations on Static Routes:** Static routes execute on a fast-path with **0 B/op and 0 allocs/op**.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
