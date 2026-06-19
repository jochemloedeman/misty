package logging

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type stub struct {
	enabled  bool
	err      error
	gotAttrs []slog.Attr
	gotGroup string
}

func (s *stub) Enabled(context.Context, slog.Level) bool  { return s.enabled }
func (s *stub) Handle(context.Context, slog.Record) error { return s.err }
func (s *stub) WithAttrs(a []slog.Attr) slog.Handler      { return &stub{gotAttrs: a} }
func (s *stub) WithGroup(n string) slog.Handler           { return &stub{gotGroup: n} }

func TestFanoutEnabledIsOR(t *testing.T) {
	if !(Fanout{&stub{enabled: false}, &stub{enabled: true}}).Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Enabled should be true when any child is enabled")
	}
	if (Fanout{&stub{enabled: false}, &stub{enabled: false}}).Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Enabled should be false when no child is enabled")
	}
}

func TestFanoutHandleJoinsErrors(t *testing.T) {
	errA := errors.New("a")
	errB := errors.New("b")
	f := Fanout{&stub{err: errA}, &stub{err: nil}, &stub{err: errB}}

	err := f.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelInfo, "msg", 0))
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("Handle should join all child errors, got %v", err)
	}

	if err := (Fanout{&stub{}, &stub{}}).Handle(context.Background(), slog.Record{}); err != nil {
		t.Fatalf("Handle with no child errors should return nil, got %v", err)
	}
}

func TestFanoutWithAttrsPropagates(t *testing.T) {
	attrs := []slog.Attr{slog.String("k", "v")}
	next, ok := (Fanout{&stub{}, &stub{}}).WithAttrs(attrs).(Fanout)
	if !ok || len(next) != 2 {
		t.Fatalf("WithAttrs should return a Fanout of the same length")
	}
	for i, h := range next {
		if got := h.(*stub).gotAttrs; len(got) != 1 || got[0].Key != "k" {
			t.Fatalf("child %d did not receive attrs: %v", i, got)
		}
	}
}

func TestFanoutWithGroupPropagates(t *testing.T) {
	next, ok := (Fanout{&stub{}, &stub{}}).WithGroup("g").(Fanout)
	if !ok || len(next) != 2 {
		t.Fatalf("WithGroup should return a Fanout of the same length")
	}
	for i, h := range next {
		if got := h.(*stub).gotGroup; got != "g" {
			t.Fatalf("child %d did not receive group: %q", i, got)
		}
	}
}
