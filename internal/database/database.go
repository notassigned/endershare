package database

import (
	"database/sql"
	"log"
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
     	value BLOB NOT NULL,
		hash BLOB NOT NULL
    )
	CREATE TABLE IF NOT EXISTS peers (
		id TEXT PRIMARY KEY,
		addrs TEXT NOT NULL,
		signature BLOB NOT NULL
	`
	if _, err := db.Exec(createTables); err != nil {
		log.Fatal(err)
	}
	return &EndershareDB{db: db}
}
