// Package gomux is an idiomatic, high-performance HTTP micro-router and
// middleware stack built directly on Go 1.27+ standard library http.ServeMux.
//
// It provides developer-friendly routing ergonomics, modular sub-routing,
// two-tier middleware chaining, typed request context helpers, and structured
// error handling—with zero external dependencies, zero regex overhead, and
// zero heap allocations on static route fast paths.
//
// # Quick Start
//
//	package main
//
//	import (
//		"log"
//		"net/http"
//
//		"github.com/vajra-labs/gomux"
//	)
//
//	func main() {
//		r := gomux.New()
//
//		// Global middleware
//		r.Use(func(next http.Handler) http.Handler {
//			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
//				log.Printf("[%s] %s", req.Method, req.URL.Path)
//				next.ServeHTTP(w, req)
//			})
//		})
//
//		// Handlers accept standard func(w, r) or func(w, r) error
//		r.Get("/", func(w http.ResponseWriter, req *http.Request) error {
//			return gomux.JSON(w, http.StatusOK, gomux.Map{
//				"message": "Welcome to gomux!",
//			})
//		})
//
//		r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
//			id := req.PathValue("id")
//			return gomux.JSON(w, http.StatusOK, gomux.Map{
//				"user_id": id,
//			})
//		})
//
//		http.ListenAndServe(":8080", r)
//	}
//
// # Middleware Architecture
//
// gomux supports two tiers of middleware:
//
// 1. Global Middleware (Use):
// Registered on the root router before route declaration. Applies to all matching
// routes and 404 (Not Found) responses.
//
// 2. Scoped Middleware (With):
// Inline middleware chains applied only to specific routes or isolated sub-groups
// without leaking to sibling routes:
//
//	r.With(authGuard).Get("/me", getProfile)
//	admin := r.With(authGuard, adminOnly)
//	admin.Get("/dashboard", getDashboard)
//
// # Sub-Routing (Route)
//
// Sub-routers can be mounted under common URL prefixes using closures:
//
//	r.Route("/api/v1", func(api *gomux.Mux) {
//		api.Get("/ping", pingHandler)
//		api.Route("/users", func(users *gomux.Mux) {
//			users.Get("/", listUsers)
//			users.Get("/{id}", getUser)
//		})
//	})
//
// # Error Handling
//
// Handlers can return standard error values or structured [HttpError] instances.
// Centralized error handling is configured via [Mux.OnError]:
//
//	r.OnError(func(w http.ResponseWriter, req *http.Request, err error) error {
//		if httpErr, ok := gomux.IsHttpError(err); ok {
//			return httpErr.ToJSON(w)
//		}
//		return gomux.JSON(w, http.StatusInternalServerError, gomux.Map{
//			"error": "Internal Server Error",
//		})
//	})
//
// # Context Helpers
//
// Type-safe helpers simplify storing and retrieving values from the request context:
//
//	req = gomux.Set(req, "userID", "user_123")
//	if userID, ok := gomux.Get[string](req, "userID"); ok {
//		// use userID safely
//	}
package gomux
