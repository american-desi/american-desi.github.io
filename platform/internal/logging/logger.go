// Package logging provides a minimal zero-dependency structured JSON logger.
package logging

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Logger writes JSON log lines to the configured writer.
type Logger struct {
	mu  sync.Mutex
	w   io.Writer
	min Level
}

// New returns a Logger writing to stdout at info level.
func New() *Logger {
	return &Logger{w: os.Stdout, min: LevelInfo}
}

// WithWriter returns a logger writing to w.
func (l *Logger) WithWriter(w io.Writer) *Logger {
	return &Logger{w: w, min: l.min}
}

// SetLevel sets the minimum level.
func (l *Logger) SetLevel(lvl Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.min = lvl
}

func (l *Logger) shouldLog(lvl Level) bool {
	order := map[Level]int{LevelDebug: 0, LevelInfo: 1, LevelWarn: 2, LevelError: 3}
	return order[lvl] >= order[l.min]
}

func (l *Logger) log(ctx context.Context, lvl Level, msg string, fields map[string]any) {
	if !l.shouldLog(lvl) {
		return
	}
	rec := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": string(lvl),
		"msg":   msg,
	}
	if rid, ok := ctx.Value(requestIDKey{}).(string); ok && rid != "" {
		rec["request_id"] = rid
	}
	for k, v := range fields {
		rec[k] = v
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	l.mu.Lock()
	_, _ = l.w.Write(append(b, '\n'))
	l.mu.Unlock()
}

func (l *Logger) Debug(ctx context.Context, msg string, f ...map[string]any) { l.log(ctx, LevelDebug, msg, merge(f)) }
func (l *Logger) Info(ctx context.Context, msg string, f ...map[string]any)  { l.log(ctx, LevelInfo, msg, merge(f)) }
func (l *Logger) Warn(ctx context.Context, msg string, f ...map[string]any)  { l.log(ctx, LevelWarn, msg, merge(f)) }
func (l *Logger) Error(ctx context.Context, msg string, f ...map[string]any) { l.log(ctx, LevelError, msg, merge(f)) }

func merge(list []map[string]any) map[string]any {
	out := map[string]any{}
	for _, m := range list {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

type requestIDKey struct{}

// WithRequestID returns a context with the request ID attached.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID extracts the request ID, if any.
func RequestID(ctx context.Context) string {
	if rid, ok := ctx.Value(requestIDKey{}).(string); ok {
		return rid
	}
	return ""
}
