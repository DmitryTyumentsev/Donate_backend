package workers

import (
	"context"
)

func (s *Service) SendFixationsInAmo(ctx context.Context) error {
	batch, err := s.fixations.BatchFixationsByPublishedNull(ctx)
	if err != nil {
		return err
	}

	return nil
}
