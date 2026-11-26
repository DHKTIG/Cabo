package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"cabo/internal/engine"
	"cabo/internal/game"
)

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

var requestSequence atomic.Uint64

func newRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), requestSequence.Add(1))
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string, retryable bool) {
	writeJSONStatus(w, status, map[string]any{
		"ok":        false,
		"error":     apiError{Code: code, Message: message, Retryable: retryable},
		"requestId": newRequestID(),
	})
}

func requireMethod(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, method := range methods {
		if r.Method == method {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不允许", false)
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any, limit int64) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "请求必须使用 application/json", false)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "请求体超过大小限制", false)
		} else {
			writeError(w, http.StatusBadRequest, "invalid_json", "请求 JSON 格式无效", false)
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求只能包含一个 JSON 值", false)
		return false
	}
	return true
}

func validHost(host string) bool {
	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		hostname = host
	}
	return hostname == "127.0.0.1" || hostname == "localhost"
}

func validOrigin(r *http.Request) bool {
	raw := r.Header.Get("Origin")
	if raw == "" {
		return true
	}
	origin, err := url.Parse(raw)
	return err == nil && origin.Scheme == "http" && strings.EqualFold(origin.Host, r.Host)
}

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(header, prefix)), []byte(s.token)) == 1
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, game.ErrNotReady):
		return http.StatusConflict
	case errors.Is(err, engine.ErrBadPlayer),
		errors.Is(err, engine.ErrBadIndex),
		errors.Is(err, engine.ErrBadPeekCount),
		errors.Is(err, engine.ErrBadClaimCount),
		errors.Is(err, engine.ErrBadTarget):
		return http.StatusBadRequest
	case errors.Is(err, engine.ErrGameOver),
		errors.Is(err, engine.ErrNotYourTurn),
		errors.Is(err, engine.ErrWrongPhase),
		errors.Is(err, engine.ErrAlreadyPeeked),
		errors.Is(err, engine.ErrNoDiscard),
		errors.Is(err, engine.ErrNoPower),
		errors.Is(err, engine.ErrNoPending),
		errors.Is(err, engine.ErrCaboLocked),
		errors.Is(err, engine.ErrLockedHand):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func errorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	default:
		return "internal_error"
	}
}

func requestErrorMessage(status int, fallback string) string {
	if status == http.StatusInternalServerError {
		return "服务内部错误"
	}
	return fallback
}

func parseActionError(err error) (int, string, string) {
	status := statusForError(err)
	message := requestErrorMessage(status, err.Error())
	return status, errorCode(status), message
}
