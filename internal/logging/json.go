// Package logging builds structured JSON logs with defensive attribute redaction.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type redactingHandler struct { next slog.Handler }

// NewJSONLogger writes structured JSON to stderr and redacts sensitive attributes.
func NewJSONLogger(level slog.Level) *slog.Logger {
	return NewJSONLoggerWithWriter(os.Stderr, level)
}

// NewJSONHandler returns a JSON handler that redacts attributes whose keys look
// sensitive. Applications should still avoid logging raw configs or credentials.
func NewJSONHandler(w io.Writer, level slog.Level, attrs ...slog.Attr) slog.Handler {
	if w == nil { w = io.Discard }
	var out slog.Handler = &redactingHandler{next: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})}
	if len(attrs) > 0 { out = out.WithAttrs(attrs) }
	return out
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}
func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
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
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(group...)}
	}
	return a
}
func sensitiveKey(key string) bool {
	k := strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(key))
	for _, marker := range []string{"password", "passwd", "token", "secret", "credential", "authorization", "privatekey", "apikey", "accesskey", "uuid"} {
		if strings.Contains(k, marker) { return true }
	}
	return false
}

func NewJSONLoggerWithWriter(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(NewJSONHandler(w, level))
}
