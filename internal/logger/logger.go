// Package logger provides something closer to zerolog's context-based logging.
//
// The allows the logger to carry attributes across the context for the duration of the request.
package logger

import (
	"context"
	"log/slog"
)

type contextKey string

const attrKey contextKey = "attrKey"

// ContextHandler implements [slog.Handler] interface and adds to the log
// record any attributes passed into the context with the [attrKey].
type ContextHandler struct {
	slog.Handler
}

// NewContextHandler creates a new instance of ContextHandler
// with `handler` as the base.
func NewContextHandler(handler slog.Handler) ContextHandler {
	return ContextHandler{Handler: handler}
}

// Handle implements [slog.Handler] interface.
func (h ContextHandler) Handle(ctx context.Context, record slog.Record) error {
	attrs, ok := ctx.Value(attrKey).([]slog.Attr)
	if !ok {
		return h.Handler.Handle(ctx, record)
	}

	// Add anything we got from the context to the current record.
	record.AddAttrs(attrs...)

	// Relinquish to the base handler.
	return h.Handler.Handle(ctx, record)
}

// Ctx creates a new context with the attached attributes.
//
// These will get logged later by the [ContextHandler] if given the resulting context.
func Ctx(ctx context.Context, toAppend ...slog.Attr) context.Context {
	attrs, ok := ctx.Value(attrKey).([]slog.Attr)
	if !ok {
		attrs = []slog.Attr{}
	}

	attrs = append(attrs, toAppend...)
	return context.WithValue(ctx, attrKey, attrs)
}
