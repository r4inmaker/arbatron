package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Storage interface {
	Connect(string) error
}

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

func NewPostgresStore(connstring string) (*PostgresStore, error) {
	pgs := new(PostgresStore)
	if err := pgs.Connect(connstring); err != nil {
		return nil, err
	}
	return pgs, nil
}
