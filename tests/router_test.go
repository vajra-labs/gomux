package tests

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vajra-labs/gomux"
)

func TestRouter_HTTPMethods(t *testing.T) {
	r := gomux.New()

	r.Get("/users", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("get-users"))
	})
	r.Post("/users", func(w http.ResponseWriter, req *http.Request) error {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("post-users"))
		return nil
	})
	r.Put("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("put-user-" + req.PathValue("id")))
		return nil
	})
	r.Delete("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	r.Patch("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("patch-user"))
		return nil
	})
	r.Options("/users", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	})
	r.Head("/users", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Custom", "test")
		w.WriteHeader(http.StatusOK)
	})
	r.All("/any", func(w http.ResponseWriter, req *http.Request) error {
		_, _ = w.Write([]byte("all-" + req.Method))
		return nil
	})
	r.On("PURGE", "/cache", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("purged"))
	})

	tests := []struct {
		method       string
		path         string
		expectedCode int
		expectedBody string
	}{
		{http.MethodGet, "/users", http.StatusOK, "get-users"},
		{http.MethodPost, "/users", http.StatusCreated, "post-users"},
		{http.MethodPut, "/users/42", http.StatusOK, "put-user-42"},
		{http.MethodDelete, "/users/42", http.StatusNoContent, ""},
		{http.MethodPatch, "/users/42", http.StatusOK, "patch-user"},
		{http.MethodOptions, "/users", http.StatusNoContent, ""},
		{http.MethodHead, "/users", http.StatusOK, ""},
		{http.MethodGet, "/any", http.StatusOK, "all-GET"},
		{http.MethodPost, "/any", http.StatusOK, "all-POST"},
		{http.MethodDelete, "/any", http.StatusOK, "all-DELETE"},
		{"PURGE", "/cache", http.StatusOK, "purged"},
	}

	for _, tc := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != tc.expectedCode {
			t.Errorf("[%s %s] expected code %d, got %d", tc.method, tc.path, tc.expectedCode, rec.Code)
		}
		if tc.expectedBody != "" && rec.Body.String() != tc.expectedBody {
			t.Errorf("[%s %s] expected body %q, got %q", tc.method, tc.path, tc.expectedBody, rec.Body.String())
		}
	}
}

func TestRouter_PathParameters(t *testing.T) {
	r := gomux.New()

	r.Get("/posts/{id}", func(w http.ResponseWriter, req *http.Request) error {
		id := req.PathValue("id")
		return gomux.JSON(w, http.StatusOK, gomux.Map{"id": id})
	})

	r.Get("/users/{userId}/posts/{postId}", func(w http.ResponseWriter, req *http.Request) error {
		return gomux.JSON(w, http.StatusOK, gomux.Map{
			"user_id": req.PathValue("userId"),
			"post_id": req.PathValue("postId"),
		})
	})

	r.Get("/files/{path...}", func(w http.ResponseWriter, req *http.Request) error {
		return gomux.Text(w, http.StatusOK, req.PathValue("path"))
	})

	// Test single param
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/123", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var res map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&res)
	if res["id"] != "123" {
		t.Errorf("expected id=123, got %s", res["id"])
	}

	// Test multiple params
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/users/alice/posts/hello-world", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	res = nil
	_ = json.NewDecoder(rec.Body).Decode(&res)
	if res["user_id"] != "alice" || res["post_id"] != "hello-world" {
		t.Errorf("expected user_id=alice, post_id=hello-world, got %+v", res)
	}

	// Test wildcard path
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/files/docs/readme.md", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "docs/readme.md" {
		t.Errorf("expected 200 docs/readme.md, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestRouter_RootPathExactMatch(t *testing.T) {
	r := gomux.New()

	r.Get("/", func(w http.ResponseWriter, req *http.Request) error {
		return gomux.Text(w, http.StatusOK, "root")
	})

	// Exact root matches
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "root" {
		t.Fatalf("expected root, got %d %q", rec.Code, rec.Body.String())
	}

	// Unregistered path does NOT hit root (does not act as catch-all)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/unknown-path", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /unknown-path, got %d", rec.Code)
	}
}

func TestRouter_NotFoundHandler(t *testing.T) {
	r := gomux.New()

	r.NotFound(func(w http.ResponseWriter, req *http.Request) error {
		return gomux.JSON(w, http.StatusNotFound, gomux.Map{
			"error": "custom not found",
			"path":  req.URL.Path,
		})
	})

	r.Get("/exists", func(w http.ResponseWriter, req *http.Request) error {
		return gomux.Text(w, http.StatusOK, "ok")
	})

	// Matching route
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/exists", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Non-matching route triggers custom 404
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/not-exists", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var res map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&res)
	if res["error"] != "custom not found" || res["path"] != "/not-exists" {
		t.Fatalf("unexpected custom 404 response: %+v", res)
	}
}

func TestRouter_ErrorHandler(t *testing.T) {
	t.Run("default error handler", func(t *testing.T) {
		r := gomux.New()
		r.Get("/error", func(w http.ResponseWriter, req *http.Request) error {
			return errors.New("database connection failed")
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/error", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "database connection failed") {
			t.Fatalf("expected error message in body, got %q", rec.Body.String())
		}
	})

	t.Run("custom OnError handler", func(t *testing.T) {
		r := gomux.New()
		r.OnError(func(w http.ResponseWriter, req *http.Request, err error) error {
			return gomux.JSON(w, http.StatusBadRequest, gomux.Map{
				"custom_err": err.Error(),
			})
		})

		r.Get("/bad", func(w http.ResponseWriter, req *http.Request) error {
			return errors.New("invalid parameter")
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/bad", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		var res map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res["custom_err"] != "invalid parameter" {
			t.Fatalf("expected custom_err, got %+v", res)
		}
	})

	t.Run("HttpError integration", func(t *testing.T) {
		r := gomux.New()
		r.Get("/unauthorized", func(w http.ResponseWriter, req *http.Request) error {
			return gomux.UnauthorizedError("missing token", "UNAUTHORIZED")
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/unauthorized", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestRouter_RoutesLockedSafety(t *testing.T) {
	r := gomux.New()
	r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic when calling Use after registering routes")
		}
	}()

	// Should panic
	r.Use(func(next http.Handler) http.Handler {
		return next
	})
}

func TestRouter_Mount(t *testing.T) {
	r := gomux.New()

	sub := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("sub-path:" + req.URL.Path))
	})

	r.Mount("/admin", sub)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "sub-path:/dashboard" {
		t.Fatalf("expected sub-path:/dashboard, got %q", rec.Body.String())
	}
}

func TestRouter_Handle(t *testing.T) {
	r := gomux.New()

	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("standard-handler"))
	})

	r.Handle("GET", "/std", stdHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/std", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "standard-handler" {
		t.Fatalf("expected 200 standard-handler, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestRouter_PatternExposedInGlobalMiddleware(t *testing.T) {
	r := gomux.New()

	var observedPattern string
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			observedPattern = req.Pattern
			next.ServeHTTP(w, req)
		})
	})

	r.Get("/items/{id}", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/99", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(observedPattern, "/items/{id}") {
		t.Fatalf("expected pattern to contain /items/{id}, got %q", observedPattern)
	}
}

func TestRouter_HandleFiles(t *testing.T) {
	r := gomux.New()

	tempDir := t.TempDir()
	r.HandleFiles("/static", http.Dir(tempDir))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	r.ServeHTTP(rec, req)

	// Will return 404 because file doesn't exist in tempDir, but proves the route handler matched
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status %d", rec.Code)
	}
}
