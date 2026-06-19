package logging

import (
	"context"
	"errors"
	"log/slog"
)

type Fanout []slog.Handler

func (f Fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f Fanout) Handle(ctx context.Context, r slog.Record) error {
	var err error
	for _, h := range f {
		err = errors.Join(err, h.Handle(ctx, r))
	}
	return err
}

func (f Fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make(Fanout, len(f))
	for i := range f {
		next[i] = f[i].WithAttrs(attrs)
	}
	return next
}

func (f Fanout) WithGroup(name string) slog.Handler {
	next := make(Fanout, len(f))
	for i := range f {
		next[i] = f[i].WithGroup(name)
	}
	return next
}
