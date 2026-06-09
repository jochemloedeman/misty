package main

import (
	"context"
	"log/slog"

	"github.com/jochemloedeman/misty/monitor"
	"go.opentelemetry.io/otel/trace"
)

const bufferSize int = 8

type RefreshRequest struct {
	monitor     monitor.Monitor
	spanContext trace.SpanContext
}

func (r RefreshRequest) Context(ctx context.Context) context.Context {
	newCtx := trace.ContextWithSpanContext(ctx, r.spanContext)
	return newCtx
}

type RefreshDispatcher struct {
	incoming chan RefreshRequest
}

func NewRefreshDispatcher() *RefreshDispatcher {
	return &RefreshDispatcher{
		incoming: make(chan RefreshRequest, bufferSize),
	}
}

func (d *RefreshDispatcher) Request(ctx context.Context, m monitor.Monitor) {
	r := RefreshRequest{
		monitor:     m,
		spanContext: trace.SpanContextFromContext(ctx),
	}

	select {
	case d.incoming <- r:
		slog.DebugContext(
			ctx,
			"immediate refresh requested",
			"monitor_id",
			r.monitor.ID,
		)
	default:
		slog.WarnContext(
			ctx,
			"immediate refresh dropped, buffer full",
			"monitor_id",
			r.monitor.ID,
		)
	}
}

func (d *RefreshDispatcher) Incoming() <-chan RefreshRequest {
	return d.incoming
}
