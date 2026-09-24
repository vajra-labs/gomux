package main

import (
	"log"
	"net/http"

	"github.com/vajra-labs/mux"
)

func main() {
	r := mux.New()

	// 1. Global Middleware (Runs on all routes & 404s)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			log.Printf("[%s] %s", req.Method, req.URL.Path)
			next.ServeHTTP(w, req)
		})
	})

	// 2. Custom 404 Handler
	r.NotFound(func(w http.ResponseWriter, req *http.Request) error {
		return mux.JSON(w, http.StatusNotFound, mux.Map{
			"error": "Route not found",
		})
	})

	// 3. Public Route (func(w, r) error)
	r.Get("/", func(w http.ResponseWriter, req *http.Request) error {
		return mux.JSON(w, http.StatusOK, mux.Map{
			"message": "Welcome to mux!",
		})
	})

	// 4. Path Parameter Route using Go native {id} matching
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
		return mux.JSON(w, http.StatusOK, mux.Map{
			"user_id": req.PathValue("id"),
		})
	})

	// 5. Auth Middleware
	authGuard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Header.Get("Authorization") != "Bearer secret" {
				_ = mux.JSON(w, http.StatusUnauthorized, mux.Map{
					"error": "Unauthorized: pass 'Authorization: Bearer secret'",
				})
				return
			}
			next.ServeHTTP(w, req)
		})
	}

	// 6. Route-Level Inline Middleware
	r.Get("/admin/dashboard", func(w http.ResponseWriter, req *http.Request) error {
		return mux.JSON(w, http.StatusOK, mux.Map{
			"secret_data": "Top secret admin dashboard content",
		})
	}, authGuard)

	// 7. Sub-Routing with Route Groups
	r.Route("/api", func(api *mux.Mux) {
		api.Get("/ping", func(w http.ResponseWriter, req *http.Request) error {
			return mux.Text(w, http.StatusOK, "pong")
		})
	})

	log.Println("🚀 Server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
