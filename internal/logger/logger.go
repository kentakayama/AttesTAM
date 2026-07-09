/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultLevel = slog.LevelInfo

type Handler struct {
	w      io.Writer
	level  slog.Leveler
	name   string
	attrs  []slog.Attr
	groups []string
	mu     *sync.Mutex
}

func New(level slog.Leveler) *slog.Logger {
	return NewWithWriter(os.Stdout, level)
}

func NewNamed(name string, level slog.Leveler) *slog.Logger {
	return NewNamedWithWriter(name, os.Stdout, level)
}

func NewWithWriter(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(NewHandler(w, level, ""))
}

func NewNamedWithWriter(name string, w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(NewHandler(w, level, name))
}

func NewHandler(w io.Writer, level slog.Leveler, name string) *Handler {
	return &Handler{
		w:     w,
		level: level,
		name:  strings.TrimSpace(name),
		mu:    &sync.Mutex{},
	}
}

func Rename(base *slog.Logger, name string) *slog.Logger {
	if base == nil {
		return nil
	}
	if h, ok := base.Handler().(*Handler); ok {
		return slog.New(h.withName(name))
	}
	return base
}

func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", raw)
	}
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	if h == nil {
		return false
	}
	return level >= h.level.Level()
}

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	if !r.Time.IsZero() {
		b.WriteString(r.Time.UTC().Format(time.RFC3339Nano))
		b.WriteByte(' ')
	}
	b.WriteByte('[')
	b.WriteString(levelLabel(r.Level))
	b.WriteString("] ")
	if h.name != "" {
		b.WriteString(h.name)
		if r.Message != "" {
			b.WriteString(": ")
		}
	}
	b.WriteString(r.Message)

	formatted := h.formatRecordAttrs(r)
	if len(formatted) > 0 {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") {
			b.WriteByte(' ')
		}
		b.WriteString(strings.Join(formatted, " "))
	}
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := h.clone()
	next.attrs = append(next.attrs, attrs...)
	return next
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := h.clone()
	next.groups = append(next.groups, name)
	return next
}

func (h *Handler) withName(name string) *Handler {
	next := h.clone()
	next.name = strings.TrimSpace(name)
	return next
}

func (h *Handler) clone() *Handler {
	if h == nil {
		return nil
	}
	next := *h
	next.attrs = append([]slog.Attr(nil), h.attrs...)
	next.groups = append([]string(nil), h.groups...)
	return &next
}

func (h *Handler) formatRecordAttrs(r slog.Record) []string {
	attrs := make([]string, 0, len(h.attrs)+r.NumAttrs())
	for _, attr := range h.attrs {
		attrs = appendAttr(attrs, h.groups, attr)
	}
	r.Attrs(func(attr slog.Attr) bool {
		attrs = appendAttr(attrs, h.groups, attr)
		return true
	})
	return attrs
}

func appendAttr(out []string, prefix []string, attr slog.Attr) []string {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return out
	}
	if attr.Value.Kind() == slog.KindGroup {
		groupPrefix := prefix
		if attr.Key != "" {
			groupPrefix = append(append([]string(nil), prefix...), attr.Key)
		}
		for _, child := range attr.Value.Group() {
			out = appendAttr(out, groupPrefix, child)
		}
		return out
	}

	keyParts := append([]string(nil), prefix...)
	if attr.Key != "" {
		keyParts = append(keyParts, attr.Key)
	}
	if len(keyParts) == 0 {
		return out
	}
	return append(out, strings.Join(keyParts, ".")+"="+formatValue(attr.Value))
}

func formatValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return quoteIfNeeded(v.String())
	case slog.KindTime:
		return v.Time().UTC().Format(time.RFC3339Nano)
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindAny:
		return formatAny(v.Any())
	default:
		return quoteIfNeeded(v.String())
	}
}

func formatAny(v any) string {
	switch x := v.(type) {
	case nil:
		return "<nil>"
	case string:
		return quoteIfNeeded(x)
	case error:
		return quoteIfNeeded(x.Error())
	case fmt.Stringer:
		return quoteIfNeeded(x.String())
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	default:
		return quoteIfNeeded(fmt.Sprint(v))
	}
}

func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		if r <= ' ' || r == '"' || r == '=' {
			return strconv.Quote(s)
		}
	}
	return s
}

func levelLabel(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return "DEBUG"
	case level < slog.LevelWarn:
		return "INFO"
	case level < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}
