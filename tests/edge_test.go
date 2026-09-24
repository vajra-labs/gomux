package tests

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/vajra-labs/mux"
)

// 1. Method Precedence: Specific method (GET) vs All (ALL)
func TestEdgeCase_MethodPrecedence(t *testing.T) {
	r := mux.New()

	r.All("/data", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("fallback-" + req.Method))
	})

	r.Get("/data", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("specific-GET"))
	})

	// GET should hit specific GET handler
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "specific-GET" {
		t.Errorf("expected specific-GET, got %q", rec.Body.String())
	}

	// POST should hit the All handler
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/data", nil)
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "fallback-POST" {
		t.Errorf("expected fallback-POST, got %q", rec.Body.String())
	}

	// DELETE should hit the All handler
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/data", nil)
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "fallback-DELETE" {
		t.Errorf("expected fallback-DELETE, got %q", rec.Body.String())
	}
}

// 2. Path Cleanup Redirect vs Custom 404 (Probe mechanism verification)
func TestEdgeCase_PathCleanupRedirectPreserved(t *testing.T) {
	r := mux.New()

	var notFoundCalled bool
	r.NotFound(func(w http.ResponseWriter, req *http.Request) error {
		notFoundCalled = true
		return mux.JSON(w, http.StatusNotFound, mux.Map{"error": "not found"})
	})

	r.Get("/a/x", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("a-x"))
	})

	// Request with redundant slash: ServeMux cleans and issues a redirect to /a/x
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/a//x", nil)
	r.ServeHTTP(rec, req)

	// Our probe mechanism must NOT mistake the redirect for a 404!
	if notFoundCalled {
		t.Fatalf("custom 404 was incorrectly invoked for a path cleanup redirect")
	}
	if rec.Code < http.StatusMultipleChoices || rec.Code >= http.StatusBadRequest {
		t.Errorf("expected redirect status, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/a/x" {
		t.Errorf("expected redirect location /a/x, got %s", rec.Header().Get("Location"))
	}
}

// 3. Deeply Nested Routes & Prefix Formatting (redundant slashes)
func TestEdgeCase_DeeplyNestedPrefixes(t *testing.T) {
	r := mux.New()

	r.Route("/api", func(api *mux.Mux) {
		api.Route("/v1", func(v1 *mux.Mux) {
			v1.Route("/auth", func(auth *mux.Mux) {
				// Empty sub-path should match /api/v1/auth
				auth.Get("", func(w http.ResponseWriter, req *http.Request) {
					_, _ = w.Write([]byte("auth-root"))
				})
				// Path with slash
				auth.Get("/login", func(w http.ResponseWriter, req *http.Request) {
					_, _ = w.Write([]byte("auth-login"))
				})
			})
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "auth-root" {
		t.Errorf("expected 200 auth-root, got %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "auth-login" {
		t.Errorf("expected 200 auth-login, got %d %q", rec.Code, rec.Body.String())
	}
}

// 4. Multi-level Nested Middleware Inheritance
func TestEdgeCase_MultiLevelMiddlewareInheritance(t *testing.T) {
	r := mux.New()

	var stack []string

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			stack = append(stack, "global")
			next.ServeHTTP(w, req)
		})
	})

	r.Route("/api", func(api *mux.Mux) {
		api.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				stack = append(stack, "api")
				next.ServeHTTP(w, req)
			})
		})

		v1 := api.With(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				stack = append(stack, "v1_scoped")
				next.ServeHTTP(w, req)
			})
		})

		v1.Get("/users", func(w http.ResponseWriter, req *http.Request) {
			stack = append(stack, "users_handler")
			w.WriteHeader(http.StatusOK)
		})

		// Sibling route on api without v1_scoped
		api.Get("/health", func(w http.ResponseWriter, req *http.Request) {
			stack = append(stack, "health_handler")
			w.WriteHeader(http.StatusOK)
		})
	})

	// Test /api/users
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	r.ServeHTTP(rec, req)

	expectedUsers := []string{"global", "api", "v1_scoped", "users_handler"}
	if !reflect.DeepEqual(stack, expectedUsers) {
		t.Fatalf("expected stack %v, got %v", expectedUsers, stack)
	}

	// Test /api/health (v1_scoped must NOT be present!)
	stack = nil
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	r.ServeHTTP(rec, req)

	expectedHealth := []string{"global", "api", "health_handler"}
	if !reflect.DeepEqual(stack, expectedHealth) {
		t.Fatalf("expected stack %v, got %v", expectedHealth, stack)
	}
}

// 5. Concurrent Requests Race Condition Test
func TestEdgeCase_ConcurrentRequestsRace(t *testing.T) {
	r := mux.New()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Tracked", req.URL.Path)
			next.ServeHTTP(w, req)
		})
	})

	r.Get("/user/{id}", func(w http.ResponseWriter, req *http.Request) error {
		return mux.JSON(w, http.StatusOK, mux.Map{"id": req.PathValue("id")})
	})

	r.Get("/static/{file...}", func(w http.ResponseWriter, req *http.Request) error {
		return mux.Text(w, http.StatusOK, req.PathValue("file"))
	})

	var wg sync.WaitGroup
	numRequests := 200

	for i := range numRequests {
		wg.Add(2)

		go func(idx int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/user/100", nil)
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("concurrent user request failed with code %d", rec.Code)
			}
		}(i)

		go func(idx int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/static/assets/app.js", nil)
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("concurrent static request failed with code %d", rec.Code)
			}
		}(i)
	}

	wg.Wait()
}

// 6. Special characters in path values
func TestEdgeCase_SpecialCharactersInPath(t *testing.T) {
	r := mux.New()

	r.Get("/lookup/{query}", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(req.PathValue("query")))
	})

	tests := []string{
		"hello-world",
		"user@company.com",
		"version_1.0.2",
		"file.tar.gz",
	}

	for _, query := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/lookup/"+query, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("failed for %q: status %d", query, rec.Code)
		}
		if rec.Body.String() != query {
			t.Errorf("expected %q, got %q", query, rec.Body.String())
		}
	}
}
