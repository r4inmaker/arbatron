package internal

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"google.golang.org/genai"
)

type PostgresStore struct {
	DB 					*sql.DB
	EmbedConfig *GenaiConfig
}

func NewPostgresStore(connstring string) (*PostgresStore, error) {
	pgs := new(PostgresStore)
	if err := pgs.Connect(connstring); err != nil {
		return nil, err
	}

	// if err := pgs.Reset(); err != nil {
	// 	return nil, err
	// }

	if err := pgs.Init(); err != nil {
		return nil, err
	}
	return pgs, nil
}

func (pgs *PostgresStore) Connect(connstring string) error {
	db, err := sql.Open("postgres", connstring)
	if err != nil {
		return fmt.Errorf("Error Opening Database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return fmt.Errorf("Database Ping Failed: %w", err)
	}

	pgs.DB = db
	return nil
}

func (pgs *PostgresStore) Init() error {
	query := `
	CREATE EXTENSION IF NOT EXISTS vector;
	CREATE TABLE IF NOT EXISTS events (
		id SERIAL PRIMARY KEY,
		event_id BIGINT UNIQUE,
		title TEXT,
		embedding vector(768),
		start_date TIMESTAMP,
		end_date TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_event_id ON events(event_id);
	`
	_, err := pgs.DB.Exec(query)
	pgs.EmbedConfig = NewGenaiConfig()

	return err
}

func (pgs *PostgresStore) RemakeIndex(ctx context.Context) error {
	query := `
    DROP INDEX IF EXISTS idx_events_lower_title, idx_events_hnsw;
    CREATE INDEX idx_events_lower_title ON events (LOWER(title));
    CREATE INDEX idx_events_hnsw ON events 
    USING hnsw (embedding vector_cosine_ops);
    `
	_, err := pgs.DB.ExecContext(ctx, query)
	return err
}

func (pgs *PostgresStore) Reset() error {
	query := `
	DROP SCHEMA IF EXISTS public CASCADE;
	CREATE SCHEMA public;
	GRANT ALL ON SCHEMA public TO postgres;
	GRANT ALL ON SCHEMA public TO public;
	`
	_, err := pgs.DB.Exec(query)
	return err
}

func (pgs *PostgresStore) BulkInsertEvents(ctx context.Context, events []DBEvent) error {
	if len(events) == 0 {
		return nil
	}

	eventIDs := make([]int64, len(events))
	titles := make([]string, len(events))
	embeddings := make([]string, len(events))
	starts := make([]time.Time, len(events))
	ends := make([]time.Time, len(events))

	for i, ev := range events {
		eventIDs[i] = int64(ev.EventID)
		titles[i] = ev.Title
		embeddings[i] = ev.Embedding
		starts[i] = time.Time(ev.StartDate)
		ends[i] = time.Time(ev.EndDate)
	}

	// This query takes the arrays, "unzips" them into rows,
	// and inserts them while ignoring any existing event_ids.
	query := `
        INSERT INTO events (event_id, title, embedding, start_date, end_date)
        SELECT * FROM unnest($1::bigint[], $2::text[], $3::vector[], $4::timestamp[], $5::timestamp[])
        ON CONFLICT (event_id) DO NOTHING;
    `

	_, err := pgs.DB.ExecContext(ctx, query,
		pq.Array(eventIDs),
		pq.Array(titles),
		pq.Array(embeddings),
		pq.Array(starts),
		pq.Array(ends),
	)

	return err
}

func (pgs *PostgresStore) GetEventCount() (int64, error) {
	query := `
		SELECT COUNT(*) FROM events;
	`
	var count int64
	if err := pgs.DB.QueryRow(query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Clustering
func (pgs *PostgresStore) ClusterAroundKeyword(ctx context.Context, client *genai.Client, keyword string, topK int, threshold float64) ([]DBEvent, error) {
	// get the embedding of the keyword
	embeddings, err := pgs.EmbedConfig.GenerateEmbeddings(ctx, client, []string{keyword})
	if err != nil {
		return nil, err
	}
	if len(embeddings) != 1 {
		return nil, fmt.Errorf("invalid length of embeding result: %d", len(embeddings))
	}

	query := `
		SELECT 
    	event_id, 
    	title, 
    	start_date,
    	embedding <=> $1 AS distance
		FROM events
		WHERE (embedding <=> $1) < $2
		ORDER BY embedding <=> $1 ASC
		LIMIT $3;
	`
	rows, err := pgs.DB.QueryContext(ctx, query, embeddings[0], threshold, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []DBEvent
	for rows.Next() {
		var ev DBEvent
		if err := rows.Scan(&ev.EventID, &ev.Title, &ev.StartDate, &ev.Embedding); err != nil {
			return nil, err
		}

		events = append(events, ev)
	}

	return events, nil
}

func (pgs *PostgresStore) ClusterAroundEventID(ctx context.Context, id int64, threshold float64) ([]DBEvent, error) {
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

func (pgs *PostgresStore) ClusterDiscovery() {

}