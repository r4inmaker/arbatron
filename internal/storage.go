package internal

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type PostgresStore struct {
	DB *sql.DB
}

func NewPostgresStore(connstring string) (*PostgresStore, error) {
	pgs := new(PostgresStore)
	if err := pgs.Connect(connstring); err != nil {
		return nil, err
	}

	if err := pgs.Reset(); err != nil {
		return nil, err
	}

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

// func (pgs *PostgresStore) InsertEvent(ctx context.Context, e DBEvent) (int64, error) {
// 	query := `
// 		INSERT INTO events (event_id, title, embedding)
// 		VALUES ($1, $2, $3);
// 	`

// 	result, err := pgs.DB.ExecContext(ctx, query)
// 	if err != nil {
// 		return 0, err
// 	}
// 	numRows, err := result.RowsAffected()
// 	if err != nil {
// 		return 0, err
// 	}

// 	return numRows, nil
// }

func (pgs *PostgresStore) BulkInsertEvents(ctx context.Context, events []DBEvent) error {
	tx, err := pgs.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	stmt, err := tx.Prepare(pq.CopyIn("events", "event_id", "title", "embedding", "start_date", "end_date"))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ev := range events {
		_, err := stmt.Exec(ev.EventID, ev.Title, ev.Embedding, time.Time(ev.StartDate), time.Time(ev.EndDate))
		if err != nil {
			return err
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	return tx.Commit()
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
