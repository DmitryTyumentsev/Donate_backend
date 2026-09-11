package postgres

import (
	"Broker_backend/services/integration/fixationservice/internal/domain/entity"
	"context"
)

func (r *Repository) BatchFixationsByPublishedNull(ctx context.Context) ([]entity.Outbox, error) {
	const op = "postgres/fixations.BatchFixationsByPublishedNull"
	query := `select o.id, o.aggregate_id, o.aggregate_type, o.event_type, o.payload, o.created_at from integration.outbox o where published_at is null`

	rows, err := r.Tx.pool.Query(ctx, query)
	if err != nil {
		return nil, MapError(op, err)
	}
	defer rows.Close()

	events := make([]entity.Outbox, 0)

	for rows.Next() {
		outbox := &entity.Outbox{}

		err = rows.Scan(
			&outbox.ObjectID,
			&outbox.ObjectType,
			&outbox.EventType,
			&outbox.Payload,
			&outbox.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		events = append(events, *outbox)
	}
	err = rows.Err() // верно понял что после rows.Next() то есть после цикла rows.Err() ловит err из цикла и мапит их?
	if err != nil {
		return nil, MapError(op, err)
	}

	return events, nil
}
