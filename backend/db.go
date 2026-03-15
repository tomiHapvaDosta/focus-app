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
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id         TEXT     PRIMARY KEY,
			title      TEXT     NOT NULL,
			area       TEXT     NOT NULL,
			duration   INTEGER,
			priority   INTEGER  CHECK(priority BETWEEN 1 AND 5),
			notes      TEXT,
			done       BOOLEAN  NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			type       TEXT     NOT NULL CHECK(type IN ('recurring', 'one-time')),
			subtype    TEXT     NOT NULL CHECK(subtype IN ('time-bound', 'flexible')),
			recurrence TEXT     NOT NULL DEFAULT '[]',
			start_time TEXT,
			user_id    TEXT     NOT NULL REFERENCES users(id)
		);

		CREATE TABLE IF NOT EXISTS completion_log (
			id           TEXT     PRIMARY KEY,
			task_id      TEXT     NOT NULL REFERENCES tasks(id),
			completed_at DATETIME NOT NULL,
			day_of_week  INTEGER  NOT NULL
		);

		CREATE TABLE IF NOT EXISTS conflict_log (
			id            TEXT     PRIMARY KEY,
			task_id_1     TEXT     NOT NULL,
			task_id_2     TEXT     NOT NULL,
			conflict_time TEXT     NOT NULL,
			day_of_week   INTEGER  NOT NULL,
			logged_at     DATETIME NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("run migration: %w", err)
	}

	return nil
}
