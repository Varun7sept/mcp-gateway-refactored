package storage

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

type NotesStore struct {
	db *sql.DB
}

func NewNotesStore(path string) (*NotesStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT DEFAULT '',
			username TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &NotesStore{db: db}, nil
}

func (ns *NotesStore) Close() error {
	return ns.db.Close()
}

func (ns *NotesStore) DB() *sql.DB {
	return ns.db
}

func (ns *NotesStore) Count() int {
	var count int
	ns.db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)
	return count
}

func init() {
	log.Println("NotesStore package initialized")
}
