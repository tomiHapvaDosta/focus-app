package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func InitDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id         TEXT     PRIMARY KEY,
		title      TEXT     NOT NULL,
		area       TEXT     NOT NULL,
		effort     INTEGER,
		priority   INTEGER  CHECK(priority BETWEEN 1 AND 5),
		notes      TEXT,
		done       BOOLEAN  NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("create tasks table: %w", err)
	}

	return nil
}
