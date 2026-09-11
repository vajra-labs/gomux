package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mux "github.com/vajra-labs/gomux"
)

// User represents a sample payload
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	r := mux.New()

	// 1. Global Middleware: Request Logger & Timer
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, req)
			log.Printf("[%s] %s | %v", req.Method, req.URL.Path, time.Since(start))
		})
	})

	// 2. Custom 404 (Not Found) Handler
	r.NotFound(func(w http.ResponseWriter, req *http.Request) error {
		return mux.JSON(w, http.StatusNotFound, mux.Map{
			"error":  "Route not found",
			"path":   req.URL.Path,
			"method": req.Method,
		})
	})

	// 3. Centralized Error Handler (Handles all returned errors)
	r.OnError(func(w http.ResponseWriter, req *http.Request, err error) error {
		if httpErr, ok := mux.IsHttpError(err); ok {
			return httpErr.ToJSON(w)
		}
		return mux.JSON(w, http.StatusInternalServerError, mux.Map{
			"error":   "Internal Server Error",
			"details": err.Error(),
		})
	})

	// 4. Public Root & Health Routes
	r.Get("/", func(w http.ResponseWriter, req *http.Request) error {
		return mux.JSON(w, http.StatusOK, mux.Map{
			"app":     "gomux-example",
			"status":  "running",
			"version": "1.0.0",
		})
	})

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) error {
		return mux.JSON(w, http.StatusOK, mux.Map{
			"status": "healthy",
			"uptime": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// 5. Auth Middleware for protected routes
	authGuard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			token := req.Header.Get("Authorization")
			if token != "Bearer secret123" {
				_ = mux.JSON(w, http.StatusUnauthorized, mux.Map{
					"error": "Unauthorized: invalid or missing Bearer token",
				})
				return
			}
			// Store authenticated user in request context using gomux.Set
			req = mux.Set(req, "userID", "usr_42")
			next.ServeHTTP(w, req)
		})
	}

	// 6. Sub-Routing with Route: /api/v1
	r.Route("/api/v1", func(api *mux.Mux) {

		// /api/v1/auth routes
		api.Route("/auth", func(auth *mux.Mux) {
			auth.Post("/login", func(w http.ResponseWriter, req *http.Request) error {
				var creds struct {
					Username string `json:"username"`
					Password string `json:"password"`
				}
				if err := mux.BindJSON(req, &creds); err != nil {
					return mux.BadRequestError("Invalid request body", "INVALID_BODY", mux.WithCause(err))
				}
				if creds.Username != "admin" || creds.Password != "pass123" {
					return mux.UnauthorizedError("Invalid credentials", "AUTH_FAILED")
				}
				return mux.JSON(w, http.StatusOK, mux.Map{
					"token": "secret123",
				})
			})

			// Inline protected route using .With()
			auth.With(authGuard).Post("/logout", func(w http.ResponseWriter, req *http.Request) error {
				userID, _ := mux.Get[string](req, "userID")
				return mux.JSON(w, http.StatusOK, mux.Map{
					"message": "Successfully logged out",
					"user_id": userID,
				})
			})
		})

		// /api/v1/users routes (Entire group protected by authGuard)
		api.Route("/users", func(users *mux.Mux) {
			users.Use(authGuard)

			// Get current authenticated user profile
			users.Get("/me", func(w http.ResponseWriter, req *http.Request) error {
				userID, ok := mux.Get[string](req, "userID")
				if !ok {
					return mux.UnauthorizedError("User not found in context", "USER_NOT_FOUND")
				}
				return mux.JSON(w, http.StatusOK, mux.Map{
					"user_id": userID,
					"name":    "Admin User",
					"role":    "admin",
				})
			})

			// Get user by ID using Go 1.22 path parameter {id}
			users.Get("/{id}", func(w http.ResponseWriter, req *http.Request) error {
				id := req.PathValue("id")
				if id != "42" {
					return mux.NotFoundError("User not found", "USER_NOT_FOUND", mux.WithMeta("searched_id", id))
				}
				return mux.JSON(w, http.StatusOK, User{
					ID:    id,
					Name:  "Gopher",
					Email: "gopher@golang.org",
				})
			})

			// Create a new user
			users.Post("/", func(w http.ResponseWriter, req *http.Request) error {
				var newUser User
				if err := mux.BindJSON(req, &newUser); err != nil {
					return mux.BadRequestError("Invalid user JSON", "INVALID_BODY")
				}
				if newUser.Name == "" || newUser.Email == "" {
					return mux.BadRequestError("name and email are required fields", "VALIDATION_FAILED")
				}
				newUser.ID = "usr_new_99"
				return mux.JSON(w, http.StatusCreated, newUser)
			})
		})
	})

	// 7. Wildcard / Catch-All Route
	r.Get("/static/{file...}", func(w http.ResponseWriter, req *http.Request) error {
		filePath := req.PathValue("file")
		return mux.JSON(w, http.StatusOK, mux.Map{
			"serving_virtual_file": filePath,
		})
	})

	// 8. Server Setup with Graceful Shutdown
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Println("🔥 gomux server listening on http://localhost:8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("Server stopped cleanly.")
}
