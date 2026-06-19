package queue

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestEnqueueRoundTrip(t *testing.T) {
	q := New[int]("test", 1)
	q.Enqueue(context.Background(), 42)

	env := <-q.C()
	if env.Payload != 42 {
		t.Fatalf("Payload = %d, want 42", env.Payload)
	}
}

func TestEnqueueDropsWhenFull(t *testing.T) {
	q := New[int]("test", 1)
	q.Enqueue(context.Background(), 1)
	q.Enqueue(context.Background(), 2) // buffer full: must drop without blocking

	env := <-q.C()
	if env.Payload != 1 {
		t.Fatalf("Payload = %d, want 1 (first enqueued)", env.Payload)
	}

	select {
	case extra := <-q.C():
		t.Fatalf("expected empty queue after drop, got %d", extra.Payload)
	default:
	}
}

func TestEnvelopeContextReattachesSpan(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01},
		SpanID:  trace.SpanID{0x02},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	q := New[int]("test", 1)
	q.Enqueue(ctx, 7)
	env := <-q.C()

	got := trace.SpanContextFromContext(env.Context(context.Background()))
	if got.TraceID() != sc.TraceID() || got.SpanID() != sc.SpanID() {
		t.Fatalf("span not reattached: trace=%v span=%v", got.TraceID(), got.SpanID())
	}
}
