// Package quickconnect provides a loopback-only web UI for starting the native
// TLS/WebSocket tunnel with just an SNI hostname and an explicit edge-IP list.
// The server key is supplied out of band through CHIMERA_TUNNEL_KEY; it is
// never accepted from the browser or returned by the API.
package quickconnect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AMINOPY/pg-core/internal/autocdn"
	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/engine/native"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

const (
	defaultSOCKSAddress = "127.0.0.1:1080"
	maxRequestBytes     = 16 << 10
)

type connectRequest struct {
	SNI     string `json:"sni"`
	EdgeIPs string `json:"edge_ips"`
}

type statusResponse struct {
	Connected bool   `json:"connected"`
	SOCKS     string `json:"socks,omitempty"`
	Message   string `json:"message,omitempty"`
}

type Server struct {
	key        string
	catalogs   map[string]*autocdn.Catalog
	catalogErr map[string]string

	mu      sync.Mutex
	adapter *native.Adapter
	addr    string
}

func NewFromEnv() (*Server, error) {
	key := strings.TrimSpace(os.Getenv("CHIMERA_TUNNEL_KEY"))
	if !validServerKey(key) {
		return nil, errors.New("CHIMERA_TUNNEL_KEY must be set to a server key of at least 24 URL-safe characters")
	}
	s := &Server{
		key:        key,
		catalogs:   make(map[string]*autocdn.Catalog),
		catalogErr: make(map[string]string),
	}
	for mode, envName := range map[string]string{
		"auto": "CHIMERA_AUTO_CANDIDATES",
		"ir":   "CHIMERA_CDN_IR_CANDIDATES",
		"us":   "CHIMERA_CDN_US_CANDIDATES",
	} {
		catalogPath := strings.TrimSpace(os.Getenv(envName))
		if catalogPath == "" {
			s.catalogErr[mode] = "candidate catalog is not configured (" + envName + ")"
			continue
		}
		data, err := os.ReadFile(catalogPath)
		if err != nil {
			s.catalogErr[mode] = "candidate catalog could not be read"
			continue
		}
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.DisallowUnknownFields()
		var catalog autocdn.Catalog
		if err := dec.Decode(&catalog); err != nil {
			s.catalogErr[mode] = "candidate catalog is invalid JSON"
		} else if err := catalog.Validate(); err != nil {
			s.catalogErr[mode] = "candidate catalog failed validation: " + err.Error()
		} else {
			s.catalogs[mode] = &catalog
		}
	}
	return s, nil
}

func validServerKey(key string) bool {
	if len(key) < 24 {
		return false
	}
	for _, r := range key {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// Handler serves the UI and a small same-origin API. Run it only on loopback.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/connect", s.handleConnect)
	mux.HandleFunc("/api/auto/select", s.handleAutoSelect)
	mux.HandleFunc("/api/disconnect", s.handleDisconnect)
	return s.localOnly(mux)
}

func (s *Server) localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = strings.Trim(r.Host, "[]")
		}
		if !strings.EqualFold(host, "localhost") {
			ip, parseErr := netip.ParseAddr(host)
			if parseErr != nil || !ip.IsLoopback() {
				http.Error(w, "loopback access only", http.StatusForbidden)
				return
			}
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			uHost := ""
			if strings.HasPrefix(origin, "http://") {
				uHost = strings.TrimPrefix(origin, "http://")
			} else if strings.HasPrefix(origin, "https://") {
				uHost = strings.TrimPrefix(origin, "https://")
			}
			if uHost == "" || !strings.EqualFold(uHost, r.Host) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	a, addr := s.adapter, s.addr
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, statusResponse{Connected: a != nil, SOCKS: addr})
}


func (s *Server) handleAutoSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid selection request"})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "ir" && mode != "us" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be auto, ir, or us"})
		return
	}
	catalog := s.catalogs[mode]
	if catalog == nil {
		message := "candidate catalog is not configured for " + mode
		if mode == "auto" {
			message = "Auto mode is not configured; set CHIMERA_AUTO_CANDIDATES to a trusted candidate catalog"
		} else if mode == "ir" {
			message = "CDN IR is not configured; set CHIMERA_CDN_IR_CANDIDATES to a trusted region catalog"
		} else if mode == "us" {
			message = "CDN US is not configured; set CHIMERA_CDN_US_CANDIDATES to a trusted region catalog"
		}
		if s.catalogErr[mode] != "" {
			message += " (" + s.catalogErr[mode] + ")"
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": message})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	routes, err := (autocdn.Scanner{Workers: 64, Timeout: 800 * time.Millisecond, Budget: autocdn.MaxPairProbes}).SelectCandidates(ctx, *catalog, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	items := make([]map[string]any, 0, len(routes))
	sniCandidates := make([]string, 0, autocdn.MaxRankedRoutes)
	cdnCandidates := make([]string, 0, autocdn.MaxRankedRoutes)
	seenSNI, seenCDN := map[string]bool{}, map[string]bool{}
	for _, route := range routes {
		items = append(items, map[string]any{
			"sni": route.SNI,
			"cdn_ip": route.CDNIP,
			"latency_ms": float64(route.Latency) / float64(time.Millisecond),
			"checked_at": route.CheckedAt,
		})
		if !seenSNI[route.SNI] && len(sniCandidates) < autocdn.MaxRankedRoutes {
			seenSNI[route.SNI] = true
			sniCandidates = append(sniCandidates, route.SNI)
		}
		if !seenCDN[route.CDNIP] && len(cdnCandidates) < autocdn.MaxRankedRoutes {
			seenCDN[route.CDNIP] = true
			cdnCandidates = append(cdnCandidates, route.CDNIP)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode": mode, "routes": items,
		"sni_candidates": sniCandidates, "cdn_candidates": cdnCandidates,
		"max_sni_candidates": autocdn.MaxCandidates,
		"max_cdn_candidates": autocdn.MaxCandidates,
		"max_pair_probes": autocdn.MaxPairProbes,
	})
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req connectRequest
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	settings := &fronting.Settings{SNI: strings.TrimSpace(req.SNI), EdgeIPs: fronting.IPList{req.EdgeIPs}}
	if err := settings.Validate(); err != nil || !settings.Enabled() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter a valid SNI hostname and at least one valid CDN edge IP"})
		return
	}

	s.mu.Lock()
	if s.adapter != nil {
		addr := s.addr
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, statusResponse{Connected: true, SOCKS: addr, Message: "already connected"})
		return
	}
	a := native.New()
	a.Fronting = settings
	profile, _ := json.Marshal(native.Profile{
		Key:       s.key,
		Host:      settings.SNI,
		Listen:    defaultSOCKSAddress,
		TimeoutMS: 8000,
	})
	if err := a.Start(r.Context(), profile); err != nil {
		s.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not start local tunnel; check whether SOCKS port 1080 is already in use"})
		return
	}
	s.adapter = a
	s.addr = a.Addr()
	addr := s.addr
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	report, err := a.Health(ctx)
	if err != nil || report.Status != engine.StatusHealthy {
		_ = a.Stop(context.Background())
		s.mu.Lock()
		if s.adapter == a {
			s.adapter = nil
			s.addr = ""
		}
		s.mu.Unlock()
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "TLS/WebSocket server check failed; verify SNI, edge IPs, server key, and server/CDN routing"})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Connected: true, SOCKS: addr, Message: "tunnel endpoint verified; configure an app to use this local SOCKS5 proxy"})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	a := s.adapter
	s.adapter, s.addr = nil, ""
	s.mu.Unlock()
	if a != nil {
		if err := a.Stop(context.Background()); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not stop tunnel cleanly"})
			return
		}
	}
	writeJSON(w, http.StatusOK, statusResponse{Connected: false, Message: "disconnected"})
}

// Serve starts the local UI. The listener is deliberately restricted to loopback.
func Serve(ctx context.Context, listen string, s *Server) error {
	if s == nil {
		return errors.New("quick-connect server is required")
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("listen address must be host:port: %w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("quick-connect UI must bind to localhost/loopback only")
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen for quick-connect UI: %w", err)
	}
	httpServer := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	done := make(chan error, 1)
	go func() { done <- httpServer.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		s.mu.Lock()
		a := s.adapter
		s.adapter, s.addr = nil, ""
		s.mu.Unlock()
		if a != nil {
			_ = a.Stop(context.Background())
		}
		return nil
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
