package workers

import (
	"Broker_backend/services/integration/fixationservice/internal/config"
	"Broker_backend/services/integration/fixationservice/internal/domain/entity"
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type FixationRepository interface {
	BatchFixationsByPublishedNull(ctx context.Context) ([]entity.Outbox, error)
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type Clock interface {
	Now() time.Time
}

type Service struct {
	cfg       *config.Config
	logger    *zap.Logger
	clock     Clock
	fixations FixationRepository
	tx        TxManager
}

func NewService(
	cfg *config.Config,
	logger *zap.Logger,
	clock Clock,
	fixation FixationRepository,
	tx TxManager,
) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		cfg:       cfg,
		logger:    logger,
		clock:     clock,
		fixations: fixation,
		tx:        tx,
	}
}

func (s *Service) ensureDeps() error {
	switch {
	case s == nil:
		return fmt.Errorf("service is nil")
	case s.cfg == nil:
		return fmt.Errorf("config is nil")
	case s.clock == nil:
		return fmt.Errorf("clock is nil")
	case s.fixations == nil:
		return fmt.Errorf("postgres is nil")
	case s.tx == nil:
		return fmt.Errorf("tx is nil")
	default:
		return nil
	}
}
