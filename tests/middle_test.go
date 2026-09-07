package tests

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vajra-labs/routemux"
)

func TestMiddleware_ExecutionOrder(t *testing.T) {
	r := routemux.New()

	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "m1_before")
			next.ServeHTTP(w, req)
			order = append(order, "m1_after")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "m2_before")
			next.ServeHTTP(w, req)
			order = append(order, "m2_after")
		})
	}

	r.Use(m1, m2)

	r.Get("/test", func(w http.ResponseWriter, req *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(rec, req)

	expected := []string{
		"m1_before",
		"m2_before",
		"handler",
		"m2_after",
		"m1_after",
	}

	if !reflect.DeepEqual(order, expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
}

func TestMiddleware_RunsOn404(t *testing.T) {
	r := routemux.New()

	var globalRan bool
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			globalRan = true
			next.ServeHTTP(w, req)
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/non-existent-page", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if !globalRan {
		t.Fatalf("expected global middleware to run even on 404")
	}
}

func TestMiddleware_ShortCircuit(t *testing.T) {
	r := routemux.New()

	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			token := req.Header.Get("Authorization")
			if token != "valid-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return // short circuit!
			}
			next.ServeHTTP(w, req)
		})
	}

	r.Use(authMiddleware)

	var handlerExecuted bool
	r.Get("/secret", func(w http.ResponseWriter, req *http.Request) {
		handlerExecuted = true
		w.WriteHeader(http.StatusOK)
	})

	// 1. Without valid token
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/secret", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if handlerExecuted {
		t.Fatalf("handler should not have executed on short-circuit")
	}

	// 2. With valid token
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/secret", nil)
	req.Header.Set("Authorization", "valid-token")
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !handlerExecuted {
		t.Fatalf("handler should have executed with valid token")
	}
}

func TestMiddleware_GroupIsolation(t *testing.T) {
	r := routemux.New()

	scopedMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Scoped", "active")
			next.ServeHTTP(w, req)
		})
	}

	// Public route
	r.Get("/public", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Scoped sub-router with middleware
	protected := r.With(scopedMw)
	protected.Get("/private", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Another public route registered on root
	r.Get("/public2", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 1. Check /public (should NOT have header)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	r.ServeHTTP(rec, req)
	if rec.Header().Get("X-Scoped") != "" {
		t.Fatalf("middleware leaked to /public")
	}

	// 2. Check /private (MUST have header)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/private", nil)
	r.ServeHTTP(rec, req)
	if rec.Header().Get("X-Scoped") != "active" {
		t.Fatalf("expected scoped middleware on /private")
	}

	// 3. Check /public2 (should NOT have header)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/public2", nil)
	r.ServeHTTP(rec, req)
	if rec.Header().Get("X-Scoped") != "" {
		t.Fatalf("middleware leaked to /public2")
	}
}

func TestMiddleware_RoutePrefixAndInheritance(t *testing.T) {
	r := routemux.New()

	var logs []string

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logs = append(logs, "global")
			next.ServeHTTP(w, req)
		})
	})

	r.Route("/api/v1", func(v1 *routemux.Mux) {
		v1.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				logs = append(logs, "v1")
				next.ServeHTTP(w, req)
			})
		})

		v1.Get("/profile", func(w http.ResponseWriter, req *http.Request) {
			logs = append(logs, "profile")
			w.WriteHeader(http.StatusOK)
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	expected := []string{"global", "v1", "profile"}
	if !reflect.DeepEqual(logs, expected) {
		t.Fatalf("expected logs %v, got %v", expected, logs)
	}
}
