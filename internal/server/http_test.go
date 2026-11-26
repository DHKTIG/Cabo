package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveRequest(handler http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1"
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder) response {
	t.Helper()
	var result response
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHTTPRejectsWrongMethods(t *testing.T) {
	handler := New()
	cases := []struct {
		method string
		path   string
		allow  string
	}{
		{method: http.MethodGet, path: "/api/new", allow: http.MethodPost},
		{method: http.MethodGet, path: "/api/action", allow: http.MethodPost},
		{method: http.MethodGet, path: "/api/ai", allow: http.MethodPost},
		{method: http.MethodPost, path: "/api/state", allow: http.MethodGet},
		{method: http.MethodPost, path: "/api/stats", allow: http.MethodGet},
	}
	for _, test := range cases {
		recorder := serveRequest(handler, test.method, test.path, "", nil)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s status = %d", test.method, test.path, recorder.Code)
		}
		if got := recorder.Header().Get("Allow"); got != test.allow {
			t.Fatalf("%s %s Allow = %q", test.method, test.path, got)
		}
		result := decodeErrorResponse(t, recorder)
		if result.Error == nil || result.Error.Code != "method_not_allowed" || result.RequestID == "" {
			t.Fatalf("invalid method error: %+v", result)
		}
	}
}

func TestHTTPRequiresJSONAndValidPayload(t *testing.T) {
	handler := New()
	recorder := serveRequest(handler, http.MethodPost, "/api/new", `{}`, nil)
	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing content type status = %d", recorder.Code)
	}

	cases := []string{
		`{`,
		`{"unknown":1}`,
		`{} {}`,
	}
	for _, body := range cases {
		recorder = serveRequest(handler, http.MethodPost, "/api/new", body, map[string]string{"Content-Type": "application/json"})
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d", body, recorder.Code)
		}
		result := decodeErrorResponse(t, recorder)
		if result.Error == nil || result.Error.Code != "invalid_json" {
			t.Fatalf("body %q error = %+v", body, result.Error)
		}
	}
}

func TestHTTPLimitsActionPayload(t *testing.T) {
	handler := New()
	body := `{"type":"` + strings.Repeat("x", 17*1024) + `"}`
	recorder := serveRequest(handler, http.MethodPost, "/api/action", body, map[string]string{"Content-Type": "application/json"})
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized action status = %d", recorder.Code)
	}
	result := decodeErrorResponse(t, recorder)
	if result.Error == nil || result.Error.Code != "payload_too_large" {
		t.Fatalf("oversized action error = %+v", result.Error)
	}
}

func TestHTTPRejectsForeignHostAndOrigin(t *testing.T) {
	handler := New()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Host = "evil.example"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("foreign Host status = %d", recorder.Code)
	}

	recorder = serveRequest(handler, http.MethodGet, "/api/state", "", map[string]string{"Origin": "http://evil.example"})
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("foreign Origin status = %d", recorder.Code)
	}
	recorder = serveRequest(handler, http.MethodGet, "/api/state", "", map[string]string{"Origin": "http://127.0.0.1"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("same-origin status = %d", recorder.Code)
	}
}

func TestHTTPBearerToken(t *testing.T) {
	t.Setenv("CABO_API_TOKEN", "test-secret")
	handler := New()
	recorder := serveRequest(handler, http.MethodGet, "/healthz", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d", recorder.Code)
	}

	recorder = serveRequest(handler, http.MethodGet, "/api/state", "", nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", recorder.Code)
	}
	recorder = serveRequest(handler, http.MethodGet, "/api/state", "", map[string]string{"Authorization": "Bearer wrong"})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d", recorder.Code)
	}
	recorder = serveRequest(handler, http.MethodGet, "/api/state", "", map[string]string{"Authorization": "Bearer test-secret"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("valid token status = %d", recorder.Code)
	}
}

func TestHTTPNotReadyError(t *testing.T) {
	handler := New()
	req := httptest.NewRequest(http.MethodPost, "/api/action", bytes.NewBufferString(`{"type":"drawDeck"}`))
	req.Host = "127.0.0.1"
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("not-ready status = %d", recorder.Code)
	}
	result := decodeErrorResponse(t, recorder)
	if result.Error == nil || result.Error.Message != "对局尚未开始" {
		t.Fatalf("not-ready error = %+v", result.Error)
	}
}
