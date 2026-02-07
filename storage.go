package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	DB *sql.DB
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

type DBEvent struct {
	ID        int
	EventID   int
	Title     string
	Embedding string
}

func (pgs *PostgresStore) Init() error {
	query := `
	CREATE EXTENSION IF NOT EXISTS vector;
	CREATE TABLE IF NOT EXISTS events (
		id SERIAL PRIMARY KEY,
		event_id BIGINT UNIQUE,
		title TEXT,
		embedding vector(768)
	);
	CREATE INDEX ON events (event_id); 
	CREATE INDEX ON events USING hnsw (embedding vector_cosine_ops);
	`
	_, err := pgs.DB.Exec(query)
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

func (pgs *PostgresStore) InsertEvent(e DBEvent) (int64, error) {
	query := `
		INSERT INTO events (event_id, title, embedding)
		VALUES ($1, $2, $3);
	`

	result, err := pgs.DB.Exec(query)
	if err != nil {
		return 0, err
	}
	numRows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return numRows, nil
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
