package internal

import (
	"context"
	"fmt"
)

func (pgs *PostgresStore) ClusterCandidates(ctx context.Context, id int64, threshold float64) ([]DBEvent, error) {
	query := `
    WITH target AS (
        SELECT embedding FROM events WHERE event_id = $1
    )
    SELECT event_id, title, start_date
    FROM events, target
    WHERE events.event_id != $1
    AND events.embedding <=> target.embedding < $2
    ORDER BY events.embedding <=> target.embedding ASC;
    `
	var exists bool
	err := pgs.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM events WHERE event_id = $1)", id).Scan(&exists)
	if !exists {
		return nil, fmt.Errorf("event_id %d does not exist in database", id)
	}

	rows, err := pgs.DB.QueryContext(ctx, query, id, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []DBEvent

	for rows.Next() {
		var e DBEvent
		if err := rows.Scan(&e.EventID, &e.Title, &e.StartDate); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, nil
}
