package main

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE COLLATE NOCASE,
	pass BLOB NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE TABLE IF NOT EXISTS sessions (
	token TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id)
);
CREATE TABLE IF NOT EXISTS invites (
	code TEXT PRIMARY KEY,
	used_by INTEGER REFERENCES users(id)
);
CREATE TABLE IF NOT EXISTS channels (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	kind TEXT NOT NULL CHECK (kind IN ('text', 'voice'))
);
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY,
	channel_id INTEGER NOT NULL REFERENCES channels(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	body TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
INSERT OR IGNORE INTO channels (name, kind) VALUES ('geral', 'text'), ('Sala', 'voice');
`

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// Uma conexão serializa tudo e evita SQLITE_BUSY; sobra para 10 pessoas.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return db, nil
}
