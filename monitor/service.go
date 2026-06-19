package monitor

import (
	"context"

	"github.com/google/uuid"
)

type Store interface {
	MonitorValidator
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Monitor, error)
	Get(ctx context.Context, userID, monitorID uuid.UUID) (Monitor, error)
	Create(ctx context.Context, m Monitor) (Monitor, error)
	Update(ctx context.Context, m Monitor) (Monitor, error)
	Delete(ctx context.Context, userID, monitorID uuid.UUID) error
}

type Service struct {
	store           Store
	scheduleRefresh func(context.Context, Monitor)
	limit           int
}

func NewService(
	store Store,
	scheduleRefresh func(context.Context, Monitor),
	limit int,
) *Service {
	return &Service{
		store:           store,
		scheduleRefresh: scheduleRefresh,
		limit:           limit,
	}
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Monitor, error) {
	return s.store.ListByUser(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID, monitorID uuid.UUID) (Monitor, error) {
	return s.store.Get(ctx, userID, monitorID)
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, location Location) (Monitor, error) {
	m, err := NewMonitor(ctx, s.store, userID, location, s.limit)
	if err != nil {
		return Monitor{}, err
	}

	created, err := s.store.Create(ctx, m)
	if err != nil {
		return Monitor{}, err
	}

	s.scheduleRefresh(ctx, created)
	return created, nil
}

func (s *Service) SetStatus(
	ctx context.Context,
	userID, monitorID uuid.UUID,
	activate bool,
) (Monitor, error) {
	m, err := s.store.Get(ctx, userID, monitorID)
	if err != nil {
		return Monitor{}, err
	}

	if activate {
		m = m.Activate()
	} else {
		m = m.Deactivate()
	}

	updated, err := s.store.Update(ctx, m)
	if err != nil {
		return Monitor{}, err
	}

	if activate {
		s.scheduleRefresh(ctx, updated)
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, userID, monitorID uuid.UUID) error {
	return s.store.Delete(ctx, userID, monitorID)
}
