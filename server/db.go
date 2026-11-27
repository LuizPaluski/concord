package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Cada posição é uma versão do banco (PRAGMA user_version). Só se acrescenta no fim.
var migrations = []string{
	`
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
`,
	// Grupos e perfis. O dono do grupo original é quem se cadastrou primeiro;
	// num banco vazio fica sem dono até o primeiro cadastro.
	`
ALTER TABLE users ADD COLUMN avatar TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN banner TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN color TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN color2 TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN effect TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN bio TEXT NOT NULL DEFAULT '';
CREATE TABLE media (
	id TEXT PRIMARY KEY,
	mime TEXT NOT NULL,
	data BLOB NOT NULL
);
CREATE TABLE servers (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	icon TEXT NOT NULL DEFAULT '',
	owner_id INTEGER REFERENCES users(id)
);
CREATE TABLE members (
	server_id INTEGER NOT NULL REFERENCES servers(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	PRIMARY KEY (server_id, user_id)
);
INSERT INTO servers (id, name, owner_id) VALUES (1, 'Concord', (SELECT min(id) FROM users));
INSERT INTO members (server_id, user_id) SELECT 1, id FROM users;
ALTER TABLE invites ADD COLUMN server_id INTEGER NOT NULL DEFAULT 1;
CREATE TABLE channels_new (
	id INTEGER PRIMARY KEY,
	server_id INTEGER NOT NULL REFERENCES servers(id),
	name TEXT NOT NULL,
	kind TEXT NOT NULL CHECK (kind IN ('text', 'voice')),
	UNIQUE (server_id, name)
);
INSERT INTO channels_new (id, server_id, name, kind) SELECT id, 1, name, kind FROM channels;
DROP TABLE channels;
ALTER TABLE channels_new RENAME TO channels;
`,
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// Uma conexão serializa tudo e evita SQLITE_BUSY; sobra para 10 pessoas.
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version >= len(migrations) {
		return nil
	}
	// Recriar a tabela de canais exige as chaves estrangeiras desligadas,
	// e esse pragma não muda dentro de transação.
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer db.Exec(`PRAGMA foreign_keys = ON`)
	for ; version < len(migrations); version++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[version]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migração %d: %w", version+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
