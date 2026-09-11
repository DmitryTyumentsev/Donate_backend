package client

import (
	"Broker_backend/services/integration/fixationservice/internal/domain/entity"
	"context"
)

func (c *Client) SendFixationAtAMOCRM(ctx context.Context, batch []entity.Outbox) (mockamocrm.Leads, error) {
	ctx, cancel, err := c.contextWithTimeout(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	return c.auth.Logout(ctx, req)
}
