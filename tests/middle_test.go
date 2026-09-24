package tests

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vajra-labs/mux"
)

func TestMiddleware_ExecutionOrder(t *testing.T) {
	r := mux.New()

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
	r := mux.New()

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
	r := mux.New()

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
	r := mux.New()

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
	r := mux.New()

	var logs []string

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logs = append(logs, "global")
			next.ServeHTTP(w, req)
		})
	})

	r.Route("/api/v1", func(v1 *mux.Mux) {
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

func TestMiddleware_InlineSingleRoute(t *testing.T) {
	r := mux.New()

	var inlineRan bool
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			inlineRan = true
			w.Header().Set("X-Inline", "true")
			next.ServeHTTP(w, req)
		})
	}

	r.Get("/with-inline", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, mw)

	r.Get("/without-inline", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 1. Route with inline middleware
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/with-inline", nil)
	r.ServeHTTP(rec, req)

	if !inlineRan || rec.Header().Get("X-Inline") != "true" {
		t.Fatalf("expected inline middleware to run on /with-inline")
	}

	// 2. Route without inline middleware should not be affected
	inlineRan = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/without-inline", nil)
	r.ServeHTTP(rec, req)

	if inlineRan || rec.Header().Get("X-Inline") != "" {
		t.Fatalf("inline middleware leaked to /without-inline")
	}
}

func TestMiddleware_InlineMultipleOrder(t *testing.T) {
	r := mux.New()

	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "inline1_before")
			next.ServeHTTP(w, req)
			order = append(order, "inline1_after")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "inline2_before")
			next.ServeHTTP(w, req)
			order = append(order, "inline2_after")
		})
	}

	m3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "inline3_before")
			next.ServeHTTP(w, req)
			order = append(order, "inline3_after")
		})
	}

	r.Get("/ordered", func(w http.ResponseWriter, req *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}, m1, m2, m3)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ordered", nil)
	r.ServeHTTP(rec, req)

	expected := []string{
		"inline1_before",
		"inline2_before",
		"inline3_before",
		"handler",
		"inline3_after",
		"inline2_after",
		"inline1_after",
	}

	if !reflect.DeepEqual(order, expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
}

func TestMiddleware_InlineWithGroupAndGlobal(t *testing.T) {
	r := mux.New()

	var order []string

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "global")
			next.ServeHTTP(w, req)
		})
	})

	r.Route("/api", func(api *mux.Mux) {
		api.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "group")
				next.ServeHTTP(w, req)
			})
		})

		inlineMw := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "inline")
				next.ServeHTTP(w, req)
			})
		}

		api.Get("/endpoint", func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "handler")
			w.WriteHeader(http.StatusOK)
		}, inlineMw)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/endpoint", nil)
	r.ServeHTTP(rec, req)

	expected := []string{"global", "group", "inline", "handler"}
	if !reflect.DeepEqual(order, expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
}

func TestMiddleware_InlineShortCircuit(t *testing.T) {
	r := mux.New()

	var handlerRan bool
	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}

	r.Get("/protected", func(w http.ResponseWriter, req *http.Request) {
		handlerRan = true
		w.WriteHeader(http.StatusOK)
	}, guard)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if handlerRan {
		t.Fatalf("handler should not have executed on short-circuit")
	}
}

func TestMiddleware_InlineAllMethods(t *testing.T) {
	r := mux.New()

	tagMiddleware := func(tag string) mux.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Add("X-Tag", tag)
				next.ServeHTTP(w, req)
			})
		}
	}

	r.Get("/get", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("GET"))
	r.Post("/post", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("POST"))
	r.Put("/put", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("PUT"))
	r.Delete("/delete", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("DELETE"))
	r.Patch("/patch", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("PATCH"))
	r.Options("/options", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("OPTIONS"))
	r.Head("/head", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("HEAD"))
	r.All("/all", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("ALL"))
	r.On("CUSTOM", "/custom", func(w http.ResponseWriter, req *http.Request) {}, tagMiddleware("CUSTOM"))
	r.Handle("GET", "/handle", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}), tagMiddleware("HANDLE"))

	cases := []struct {
		method string
		path   string
		tag    string
	}{
		{http.MethodGet, "/get", "GET"},
		{http.MethodPost, "/post", "POST"},
		{http.MethodPut, "/put", "PUT"},
		{http.MethodDelete, "/delete", "DELETE"},
		{http.MethodPatch, "/patch", "PATCH"},
		{http.MethodOptions, "/options", "OPTIONS"},
		{http.MethodHead, "/head", "HEAD"},
		{http.MethodGet, "/all", "ALL"},
		{"CUSTOM", "/custom", "CUSTOM"},
		{http.MethodGet, "/handle", "HANDLE"},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(c.method, c.path, nil)
		r.ServeHTTP(rec, req)
		if rec.Header().Get("X-Tag") != c.tag {
			t.Errorf("[%s %s] expected X-Tag=%q, got %q", c.method, c.path, c.tag, rec.Header().Get("X-Tag"))
		}
	}
}

func TestMiddleware_InlineHandleFilesAndMount(t *testing.T) {
	r := mux.New()

	mwFiles := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Files-Mw", "active")
			next.ServeHTTP(w, req)
		})
	}
	mwMount := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Mount-Mw", "active")
			next.ServeHTTP(w, req)
		})
	}

	tempDir := t.TempDir()
	r.HandleFiles("/static", http.Dir(tempDir), mwFiles)

	sub := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mounted"))
	})
	r.Mount("/sub", sub, mwMount)

	// Test HandleFiles with inline middleware
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/static/file.txt", nil)
	r.ServeHTTP(rec, req)
	if rec.Header().Get("X-Files-Mw") != "active" {
		t.Fatalf("expected X-Files-Mw on HandleFiles")
	}

	// Test Mount with inline middleware
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
	r.ServeHTTP(rec, req)
	if rec.Header().Get("X-Mount-Mw") != "active" {
		t.Fatalf("expected X-Mount-Mw on Mount")
	}
}
