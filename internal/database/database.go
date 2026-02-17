package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type EndershareDB struct {
	db *sql.DB
}

// The node table stores key-value pairs for this node
// The data table stores data replicated between nodes
func Create() *EndershareDB {
	db, err := sql.Open("sqlite3", "./endershare.db")
	if err != nil {
		log.Fatal(err)
	}

	createTables := `
	CREATE TABLE IF NOT EXISTS node (
        key TEXT NOT NULL PRIMARY KEY,
        value TEXT
    );
    CREATE TABLE IF NOT EXISTS data (
    	key BLOB PRIMARY KEY,
     	value BLOB NULL,
		size INTEGER NOT NULL,
		hash BLOB NOT NULL,
		in_current BOOLEAN DEFAULT 1,
		download_progress INTEGER DEFAULT 0,
		folder_tag BLOB NULL
    );
	CREATE INDEX IF NOT EXISTS idx_data_hash ON data(hash);
	CREATE INDEX IF NOT EXISTS idx_data_folder_tag ON data(folder_tag);
	CREATE TABLE IF NOT EXISTS peers (
		peer_id TEXT PRIMARY KEY,
		addrs TEXT NULL
	);
	CREATE TABLE IF NOT EXISTS updates (
		update_id INTEGER PRIMARY KEY,
		signed_update_json TEXT NOT NULL
	);
	`
	if _, err := db.Exec(createTables); err != nil {
		log.Fatal(err)
	}
	return &EndershareDB{db: db}
}
