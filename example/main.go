package main

import (
	"log"
	"net/http"

	"github.com/vajra-labs/gomux"
)

func main() {
	r := gomux.New()

	// 1. Global Middleware (Runs on all routes & 404s)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[%s] %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})

	// 2. Custom 404 Handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) error {
		return gomux.JSON(w, http.StatusNotFound, gomux.Map{
			"error": "Route not found",
		})
	})

	// 3. Public Route (func(w, r) error)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) error {
		return gomux.JSON(w, http.StatusOK, gomux.Map{
			"message": "Welcome to gomux!",
		})
	})

	// 4. Path Parameter Route using Go native {id} matching
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) error {
		return gomux.JSON(w, http.StatusOK, gomux.Map{
			"user_id": r.PathValue("id"),
		})
	})

	// 5. Auth Middleware
	authGuard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer secret" {
				_ = gomux.JSON(w, http.StatusUnauthorized, gomux.Map{
					"error": "UnAuthorized: pass 'Authorization: Bearer secret'",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// 6. Route-Level Inline Middleware
	r.Get("/admin/dashboard", func(w http.ResponseWriter, r *http.Request) error {
		return gomux.JSON(w, http.StatusOK, gomux.Map{
			"secret_data": "Top secret admin dashboard content",
		})
	}, authGuard)

	// 7. Sub-Routing with Route Groups
	r.Route("/api", func(api *gomux.Mux) {
		api.Get("/ping", func(w http.ResponseWriter, r *http.Request) error {
			return gomux.Text(w, http.StatusOK, "pong")
		})
	})

	log.Println("🚀 Server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
