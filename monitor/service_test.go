package monitor

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeStore struct {
	count     int
	exists    bool
	getResult Monitor
	getErr    error
	createErr error
	updateErr error
	deleteErr error

	created *Monitor
	updated *Monitor
	deleted bool
}

func (f *fakeStore) CountByUser(context.Context, uuid.UUID) (int, error) {
	return f.count, nil
}

func (f *fakeStore) LocationExistsByUser(context.Context, uuid.UUID, float64, float64) (bool, error) {
	return f.exists, nil
}

func (f *fakeStore) ListByUser(context.Context, uuid.UUID) ([]Monitor, error) {
	return nil, nil
}

func (f *fakeStore) Get(context.Context, uuid.UUID, uuid.UUID) (Monitor, error) {
	return f.getResult, f.getErr
}

func (f *fakeStore) Create(_ context.Context, m Monitor) (Monitor, error) {
	if f.createErr != nil {
		return Monitor{}, f.createErr
	}
	f.created = &m
	return m, nil
}

func (f *fakeStore) Update(_ context.Context, m Monitor) (Monitor, error) {
	if f.updateErr != nil {
		return Monitor{}, f.updateErr
	}
	f.updated = &m
	return m, nil
}

func (f *fakeStore) Delete(context.Context, uuid.UUID, uuid.UUID) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = true
	return nil
}

type refreshSpy struct {
	calls []Monitor
}

func (s *refreshSpy) schedule(_ context.Context, m Monitor) {
	s.calls = append(s.calls, m)
}

func TestServiceCreateSchedulesRefresh(t *testing.T) {
	ctx := t.Context()
	store := &fakeStore{count: 1}
	spy := &refreshSpy{}
	svc := NewService(store, spy.schedule, 5)

	created, err := svc.Create(ctx, uuid.New(), Location{Name: "Test", Lat: 52.0, Lon: 5.0})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if store.created == nil {
		t.Fatal("Create() did not persist the monitor")
	}
	if len(spy.calls) != 1 {
		t.Fatalf("scheduleRefresh called %d times, want 1", len(spy.calls))
	}
	if spy.calls[0].ID != created.ID {
		t.Fatalf("scheduleRefresh got monitor %s, want %s", spy.calls[0].ID, created.ID)
	}
}

func TestServiceCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		store   *fakeStore
		wantErr error
	}{
		{
			name:    "limit reached",
			store:   &fakeStore{count: 5},
			wantErr: ErrLimitReached,
		},
		{
			name:    "duplicate location",
			store:   &fakeStore{count: 1, exists: true},
			wantErr: ErrDuplicateLocation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &refreshSpy{}
			svc := NewService(tt.store, spy.schedule, 5)

			_, err := svc.Create(t.Context(), uuid.New(), Location{Name: "Test"})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, tt.wantErr)
			}
			if len(spy.calls) != 0 {
				t.Fatalf("scheduleRefresh called %d times on failure, want 0", len(spy.calls))
			}
		})
	}
}

func TestServiceSetStatus(t *testing.T) {
	tests := []struct {
		name         string
		activate     bool
		wantSchedule int
	}{
		{name: "activate schedules refresh", activate: true, wantSchedule: 1},
		{name: "deactivate does not schedule", activate: false, wantSchedule: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{getResult: Monitor{ID: uuid.New(), IsActive: !tt.activate}}
			spy := &refreshSpy{}
			svc := NewService(store, spy.schedule, 5)

			_, err := svc.SetStatus(t.Context(), uuid.New(), store.getResult.ID, tt.activate)
			if err != nil {
				t.Fatalf("SetStatus() unexpected error: %v", err)
			}
			if store.updated == nil {
				t.Fatal("SetStatus() did not persist the update")
			}
			if len(spy.calls) != tt.wantSchedule {
				t.Fatalf("scheduleRefresh called %d times, want %d", len(spy.calls), tt.wantSchedule)
			}
		})
	}
}

func TestServiceSetStatusNotFound(t *testing.T) {
	store := &fakeStore{getErr: ErrNotFound}
	spy := &refreshSpy{}
	svc := NewService(store, spy.schedule, 5)

	_, err := svc.SetStatus(t.Context(), uuid.New(), uuid.New(), true)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetStatus() error = %v, want %v", err, ErrNotFound)
	}
	if len(spy.calls) != 0 {
		t.Fatalf("scheduleRefresh called %d times on not-found, want 0", len(spy.calls))
	}
}

func TestServiceDeleteDoesNotSchedule(t *testing.T) {
	store := &fakeStore{}
	spy := &refreshSpy{}
	svc := NewService(store, spy.schedule, 5)

	if err := svc.Delete(t.Context(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}
	if !store.deleted {
		t.Fatal("Delete() did not reach the store")
	}
	if len(spy.calls) != 0 {
		t.Fatalf("scheduleRefresh called %d times on delete, want 0", len(spy.calls))
	}
}
