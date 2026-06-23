package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	errDeliver     = errors.New("deliver failed")
	errMarkSent    = errors.New("mark sent failed")
	errMarkExpired = errors.New("mark expired failed")
)

type stubOutbox struct {
	markSentErr    error
	markExpiredErr error

	markSentID    uuid.UUID
	markSentAt    time.Time
	markSentCalls int

	markExpiredID    uuid.UUID
	markExpiredCalls int
}

func (s *stubOutbox) ListUnsent(context.Context) ([]Fog, error) {
	return nil, nil
}

func (s *stubOutbox) Find(context.Context, uuid.UUID) (Fog, bool, error) {
	return Fog{}, false, nil
}

func (s *stubOutbox) MarkSent(_ context.Context, id uuid.UUID, sentAt time.Time) error {
	s.markSentCalls++
	s.markSentID = id
	s.markSentAt = sentAt
	return s.markSentErr
}

func (s *stubOutbox) MarkExpired(_ context.Context, id uuid.UUID) error {
	s.markExpiredCalls++
	s.markExpiredID = id
	return s.markExpiredErr
}

func TestDeliverOne(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		expired        bool
		deliverErr     error
		markSentErr    error
		markExpiredErr error

		wantErrIs       error
		wantDeliver     bool
		wantMarkSent    bool
		wantMarkExpired bool
	}{
		{
			name:            "expired marks expired and skips delivery",
			expired:         true,
			wantMarkExpired: true,
		},
		{
			name:            "expired propagates MarkExpired error",
			expired:         true,
			markExpiredErr:  errMarkExpired,
			wantErrIs:       errMarkExpired,
			wantMarkExpired: true,
		},
		{
			name:         "delivered marks sent",
			wantDeliver:  true,
			wantMarkSent: true,
		},
		{
			name:        "deliver failure skips MarkSent",
			deliverErr:  errDeliver,
			wantErrIs:   errDeliver,
			wantDeliver: true,
		},
		{
			name:         "MarkSent failure propagates after delivery",
			markSentErr:  errMarkSent,
			wantErrIs:    errMarkSent,
			wantDeliver:  true,
			wantMarkSent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &stubOutbox{
				markSentErr:    tt.markSentErr,
				markExpiredErr: tt.markExpiredErr,
			}

			var deliverCalls int
			deliver := func(context.Context, Fog) error {
				deliverCalls++
				return tt.deliverErr
			}

			n, err := NewNotifier(out, deliver, func() time.Time { return now })
			if err != nil {
				t.Fatalf("NewNotifier() error = %v", err)
			}

			fogEnd := now.Add(time.Hour)
			if tt.expired {
				fogEnd = now.Add(-time.Hour)
			}
			notif := Fog{ID: uuid.New(), RecipientID: uuid.New(), FogEnd: fogEnd}

			err = n.deliverOne(t.Context(), notif)

			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("deliverOne() error = %v, want errors.Is %v", err, tt.wantErrIs)
				}
			} else if err != nil {
				t.Fatalf("deliverOne() error = %v, want nil", err)
			}

			if got := deliverCalls > 0; got != tt.wantDeliver {
				t.Errorf("deliver called = %v, want %v", got, tt.wantDeliver)
			}

			if got := out.markSentCalls > 0; got != tt.wantMarkSent {
				t.Errorf("MarkSent called = %v, want %v", got, tt.wantMarkSent)
			}
			if tt.wantMarkSent {
				if out.markSentID != notif.ID {
					t.Errorf("MarkSent id = %v, want %v", out.markSentID, notif.ID)
				}
				if !out.markSentAt.Equal(now) {
					t.Errorf("MarkSent sentAt = %v, want %v", out.markSentAt, now)
				}
			}

			if got := out.markExpiredCalls > 0; got != tt.wantMarkExpired {
				t.Errorf("MarkExpired called = %v, want %v", got, tt.wantMarkExpired)
			}
			if tt.wantMarkExpired && out.markExpiredID != notif.ID {
				t.Errorf("MarkExpired id = %v, want %v", out.markExpiredID, notif.ID)
			}
		})
	}
}
