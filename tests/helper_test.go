package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vajra-labs/mux"
)

type userPayload struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestHelper_JSON(t *testing.T) {
	rec := httptest.NewRecorder()
	data := mux.Map{"status": "ok", "count": 42}

	err := mux.JSON(rec, http.StatusOK, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains([]byte(rec.Header().Get("Content-Type")), []byte("application/json")) {
		t.Errorf("expected application/json Content-Type, got %s", rec.Header().Get("Content-Type"))
	}

	var res map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["status"] != "ok" || res["count"].(float64) != 42 {
		t.Errorf("unexpected body content: %+v", res)
	}
}

func TestHelper_BindJSON(t *testing.T) {
	t.Run("valid JSON body", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"Alice","email":"alice@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)

		var payload userPayload
		err := mux.BindJSON(req, &payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payload.Name != "Alice" || payload.Email != "alice@example.com" {
			t.Errorf("unexpected payload: %+v", payload)
		}
	})

	t.Run("empty or nil body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		var payload userPayload
		err := mux.BindJSON(req, &payload)
		if err == nil {
			t.Fatalf("expected error for empty body, got nil")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid json}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		var payload userPayload
		err := mux.BindJSON(req, &payload)
		if err == nil {
			t.Fatalf("expected error for invalid json, got nil")
		}
	})
}

func TestHelper_Text(t *testing.T) {
	rec := httptest.NewRecorder()
	err := mux.Text(rec, http.StatusOK, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", rec.Body.String())
	}
}

func TestHelper_Query(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/search?q=golang&page=2", nil)
	if mux.Query(req, "q") != "golang" {
		t.Errorf("expected q=golang, got %s", mux.Query(req, "q"))
	}
	if mux.Query(req, "page") != "2" {
		t.Errorf("expected page=2, got %s", mux.Query(req, "page"))
	}
	if mux.Query(req, "missing") != "" {
		t.Errorf("expected empty for missing param, got %s", mux.Query(req, "missing"))
	}
}

func TestHelper_Redirect(t *testing.T) {
	t.Run("default 302 Found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/old", nil)
		_ = mux.Redirect(rec, req, "/new")
		if rec.Code != http.StatusFound {
			t.Errorf("expected 302, got %d", rec.Code)
		}
		if rec.Header().Get("Location") != "/new" {
			t.Errorf("expected Location /new, got %s", rec.Header().Get("Location"))
		}
	})

	t.Run("custom 301 Moved Permanently", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/old", nil)
		_ = mux.Redirect(rec, req, "/new", http.StatusMovedPermanently)
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("expected 301, got %d", rec.Code)
		}
	})
}

func TestHelper_NoContent(t *testing.T) {
	rec := httptest.NewRecorder()
	_ = mux.NoContent(rec, http.StatusNoContent)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", rec.Body.String())
	}
}

func TestHelper_ContextSetGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Nil request check
	if val, ok := mux.Get[string](nil, "any"); ok || val != "" {
		t.Errorf("expected false for nil request, got %v, %v", val, ok)
	}

	// Missing key check
	if val, ok := mux.Get[string](req, "missing"); ok || val != "" {
		t.Errorf("expected false for missing key, got %v, %v", val, ok)
	}

	// Set string value
	req = mux.Set(req, "userID", "usr_12345")
	if val, ok := mux.Get[string](req, "userID"); !ok || val != "usr_12345" {
		t.Errorf("expected usr_12345, got %v, ok=%v", val, ok)
	}

	// Set custom struct / typed value
	type Session struct {
		Role string
		ID   int
	}
	req = mux.Set(req, "session", Session{Role: "admin", ID: 99})

	session, ok := mux.Get[Session](req, "session")
	if !ok || session.Role != "admin" || session.ID != 99 {
		t.Errorf("expected valid session, got %+v, ok=%v", session, ok)
	}

	// Type mismatch check (stored Session, querying int)
	if _, ok := mux.Get[int](req, "session"); ok {
		t.Errorf("expected false on type mismatch, got true")
	}
}

func TestErrorx(t *testing.T) {
	t.Run("constructor and properties", func(t *testing.T) {
		cause := errors.New("sql: no rows")
		err := mux.NotFoundError(
			"user was not found",
			"USER_NOT_FOUND",
			mux.WithCause(cause),
			mux.WithMeta("user_id", "42"),
		)

		if err.Status != http.StatusNotFound {
			t.Errorf("expected 404, got %d", err.Status)
		}
		if err.Code != "USER_NOT_FOUND" {
			t.Errorf("expected USER_NOT_FOUND, got %s", err.Code)
		}
		if err.Error() != "user was not found" {
			t.Errorf("expected message 'user was not found', got %s", err.Error())
		}
		if err.Cause() != cause {
			t.Errorf("expected cause %v, got %v", cause, err.Cause())
		}
		if !errors.Is(err, cause) {
			t.Errorf("expected errors.Is to unwrap cause")
		}
		if err.Meta["user_id"] != "42" {
			t.Errorf("expected meta user_id=42, got %v", err.Meta["user_id"])
		}
	})

	t.Run("IsHttpError helper", func(t *testing.T) {
		httpErr := mux.BadRequestError("invalid email", "INVALID_EMAIL")
		parsed, ok := mux.IsHttpError(httpErr)
		if !ok || parsed == nil || parsed.Status != http.StatusBadRequest {
			t.Fatalf("expected IsHttpError to succeed for HttpError")
		}

		stdErr := errors.New("normal error")
		_, ok = mux.IsHttpError(stdErr)
		if ok {
			t.Fatalf("expected IsHttpError to return false for standard error")
		}

		_, ok = mux.IsHttpError(nil)
		if ok {
			t.Fatalf("expected IsHttpError to return false for nil")
		}
	})

	t.Run("ToJSON helper", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := mux.ForbiddenError("access denied", "FORBIDDEN")
		_ = err.ToJSON(rec)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
		var body map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&body)
		if body["error"] != "Forbidden" || body["code"] != "FORBIDDEN" {
			t.Errorf("unexpected json body: %+v", body)
		}
	})
}
