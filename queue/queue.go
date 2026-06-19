package queue

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type Envelope[T any] struct {
	Payload     T
	spanContext trace.SpanContext
}

func (e Envelope[T]) Context(ctx context.Context) context.Context {
	return trace.ContextWithSpanContext(ctx, e.spanContext)
}

type Queue[T any] struct {
	name     string
	incoming chan Envelope[T]
}

func New[T any](name string, buffer int) *Queue[T] {
	return &Queue[T]{
		name:     name,
		incoming: make(chan Envelope[T], buffer),
	}
}

func (q *Queue[T]) Enqueue(ctx context.Context, payload T) {
	e := Envelope[T]{
		Payload:     payload,
		spanContext: trace.SpanContextFromContext(ctx),
	}

	select {
	case q.incoming <- e:
		slog.DebugContext(ctx, "enqueued", "queue", q.name)
	default:
		slog.WarnContext(ctx, "enqueue dropped, buffer full", "queue", q.name)
	}
}

func (q *Queue[T]) C() <-chan Envelope[T] {
	return q.incoming
}
