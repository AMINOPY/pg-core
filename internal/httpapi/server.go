// Package httpapi exposes a small, authenticated HTTP boundary for integrating
// the core into server-side applications. It does not execute connectors.
package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/AMINOPY/pg-core/internal/capability"
)

const maxRequestBytes = capability.MaxManifestBytes

type Server struct {
	apiKey         string
	allowedOrigins map[string]struct{}
	mux            *http.ServeMux
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type validationResponse struct {
	SchemaVersion string              `json:"schema_version"`
	Valid         bool                `json:"valid"`
	ConnectorID   string              `json:"connector_id,omitempty"`
	Version       string              `json:"connector_version,omitempty"`
	Capabilities  []capabilitySummary `json:"capabilities,omitempty"`
}

type capabilitySummary struct {
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Risk        string   `json:"risk"`
	Execution   string   `json:"execution"`
	Permissions []string `json:"permissions"`
}

// New requires a non-empty server-side API key. allowedOrigins must contain
// exact origins (scheme + host + optional port); an empty list disables CORS.
func New(apiKey string, allowedOrigins []string) (http.Handler, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("API key must be configured")
	}
	s := &Server{apiKey: apiKey, allowedOrigins: make(map[string]struct{}), mux: http.NewServeMux()}
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			s.allowedOrigins[origin] = struct{}{}
		}
	}
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.Handle("POST /v1/manifests/validate", s.authenticate(http.HandlerFunc(s.validateManifest)))
	return s.securityHeaders(s.cors(s.mux)), nil
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "api_version": "v1"})
}

func (s *Server) validateManifest(w http.ResponseWriter, r *http.Request) {
	mediaType, _, mediaTypeErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaTypeErr != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "CONTENT_TYPE_REQUIRED", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "manifest exceeds the request size limit")
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "could not read request body")
		return
	}
	manifest, err := capability.ParseManifest(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_MANIFEST", "manifest failed validation")
		return
	}
	response := validationResponse{
		SchemaVersion: capability.SchemaVersion,
		Valid: true,
		ConnectorID: manifest.ConnectorID,
		Version: manifest.ConnectorVersion,
		Capabilities: make([]capabilitySummary, 0, len(manifest.Capabilities)),
	}
	for _, item := range manifest.Capabilities {
		perms := append([]string(nil), item.Permissions...)
		response.Capabilities = append(response.Capabilities, capabilitySummary{
			ID: item.ID, Version: item.Version, Risk: item.Risk,
			Execution: item.Execution, Permissions: perms,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "valid bearer token required")
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		if len(provided) != len(s.apiKey) || subtle.ConstantTimeCompare([]byte(provided), []byte(s.apiKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "valid bearer token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := s.allowedOrigins[origin]; !ok {
			writeError(w, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "origin is not allowed")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("httpapi: response encoding failed: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var response errorEnvelope
	response.Error.Code = code
	response.Error.Message = message
	writeJSON(w, status, response)
}

// RecommendedServer wraps a handler with conservative HTTP timeouts.
// Deploy behind HTTPS; do not expose the API directly over public plaintext HTTP.
func RecommendedServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: addr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 16 << 10,
	}
}
