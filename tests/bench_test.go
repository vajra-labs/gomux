package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/vajra-labs/gomux"
)

// Helper to format bytes nicely
func formatMemory(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// =========================================================================
// 1. ROUTER MEMORY FOOTPRINT BENCHMARKS (RAM used by router at scale)
// =========================================================================

func TestRouter_MemoryFootprint(t *testing.T) {
	counts := []int{100, 1000, 5000, 10000}
	fmt.Println("\n================ ROUTER RAM FOOTPRINT ================")
	for _, count := range counts {
		runtime.GC()
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		r := gomux.New()
		for i := range count {
			switch i % 3 {
			case 0:
				r.Get(fmt.Sprintf("/api/v1/resource%d/items", i), func(w http.ResponseWriter, req *http.Request) {})
			case 1:
				r.Get(fmt.Sprintf("/api/v1/users%d/{id}/posts/{postID}", i), func(w http.ResponseWriter, req *http.Request) {})
			default:
				r.Get(fmt.Sprintf("/static/files%d/{path...}", i), func(w http.ResponseWriter, req *http.Request) {})
			}
		}

		runtime.ReadMemStats(&m2)
		alloc := m2.TotalAlloc - m1.TotalAlloc
		fmt.Printf("Routes: %-6d | RAM Consumed: %-10s | Per-Route: ~%d bytes\n",
			count, formatMemory(alloc), alloc/uint64(count))
	}
	fmt.Println("======================================================")
}

// =========================================================================
// 2. CPU & REQUEST-TIME MEMORY BENCHMARKS (Latency & Allocations per request)
// =========================================================================
// 2. CPU & REQUEST-TIME MEMORY BENCHMARKS (Latency & Allocations per request)
// =========================================================================

func BenchmarkMux_StaticRoute(b *testing.B) {
	r := gomux.New()
	r.Get("/api/v1/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMux_SingleParamRoute(b *testing.B) {
	r := gomux.New()
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		_ = req.PathValue("id")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMux_MultiParamRoute(b *testing.B) {
	r := gomux.New()
	r.Get("/users/{userId}/posts/{postId}", func(w http.ResponseWriter, req *http.Request) {
		_ = req.PathValue("userId")
		_ = req.PathValue("postId")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/100/posts/500", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMux_WithMiddlewarePipeline(b *testing.B) {
	r := gomux.New()

	// Global middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Global", "true")
			next.ServeHTTP(w, req)
		})
	})

	// Scoped .With middleware
	authMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Auth", "ok")
			next.ServeHTTP(w, req)
		})
	}

	r.With(authMw).Get("/api/secure/profile", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/secure/profile", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMux_JSONResponse(b *testing.B) {
	r := gomux.New()
	data := gomux.Map{"status": "ok", "message": "hello world", "id": 12345}

	r.Get("/api/json", func(w http.ResponseWriter, req *http.Request) error {
		return gomux.JSON(w, http.StatusOK, data)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/json", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMux_InlineMiddleware(b *testing.B) {
	r := gomux.New()
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req)
		})
	}
	r.Get("/api/v1/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, mw)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}
