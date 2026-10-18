// Package logging builds structured JSON logs with defensive attribute redaction.
package logging

import (
	"context"
	"log/slog"
	"strings"
)

type redactingHandler struct { next slog.Handler }

func NewJSONLogger(level slog.Level, attrs ...slog.Attr) *slog.Logger {
	h := slog.NewJSONHandler(nilWriter{}, &slog.HandlerOptions{Level: level})
	_ = h
	return slog.New(NewJSONHandler(defaultWriter{}, level, attrs...))
}

// NewJSONHandler returns a JSON handler that redacts attributes whose keys look
// sensitive. Applications should still avoid logging raw configs or credentials.
func NewJSONHandler(w interface{ Write([]byte) (int, error) }, level slog.Level, attrs ...slog.Attr) slog.Handler {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	var out slog.Handler = &redactingHandler{next: h}
	if len(attrs) > 0 { out = out.WithAttrs(attrs) }
	return out
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}
func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	var clean slog.Record
	clean = slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool { clean.AddAttrs(redact(a)); return true })
	return h.next.Handle(ctx, clean)
}
func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs { out[i] = redact(a) }
	return &redactingHandler{next: h.next.WithAttrs(out)}
}
func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}

func redact(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	if sensitiveKey(a.Key) { return slog.String(a.Key, "[REDACTED]") }
	if a.Value.Kind() == slog.KindGroup {
		group := a.Value.Group()
		for i := range group { group[i] = redact(group[i]) }
		return slog.Group(a.Key, attrsToAny(group)...)
	}
	return a
}

func attrsToAny(attrs []slog.Attr) []any {
	out := make([]any, len(attrs))
	for i, a := range attrs { out[i] = a }
	return out
}
func sensitiveKey(key string) bool {
	k := strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(key))
	for _, marker := range []string{"password", "passwd", "token", "secret", "credential", "authorization", "privatekey", "apikey", "accesskey", "uuid"} {
		if strings.Contains(k, marker) { return true }
	}
	return false
}

// NewJSONLoggerWithWriter is the preferred constructor for applications.
func NewJSONLoggerWithWriter(w interface{ Write([]byte) (int, error) }, level slog.Level) *slog.Logger {
	return slog.New(NewJSONHandler(w, level))
}

type nilWriter struct{}
func (nilWriter) Write(p []byte) (int, error) { return len(p), nil }
type defaultWriter struct{}
func (defaultWriter) Write(p []byte) (int, error) { return len(p), nil }
