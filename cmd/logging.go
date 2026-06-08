package main

import (
	"context"
	"errors"
	"log/slog"
)

type fanout []slog.Handler

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	var err error
	for _, h := range f {
		err = errors.Join(err, h.Handle(ctx, r))
	}
	return err
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	newFanout := make(fanout, len(f))
	for i := range len(f) {
		newFanout[i] = f[i].WithAttrs(attrs)
	}
	return newFanout
}

func (f fanout) WithGroup(name string) slog.Handler {
	newFanout := make(fanout, len(f))
	for i := range len(f) {
		newFanout[i] = f[i].WithGroup(name)
	}
	return newFanout
}
