package database

import (
	"database/sql"
	"encoding/base64"
	"log"

	"github.com/notassigned/endershare/internal/crypto"
)

type EndershareDB struct {
	db *sql.DB
}

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
    	hash BLOB PRIMARY KEY,
     	content BLOB NOT NULL
    )
	`
	if _, err := db.Exec(createTables); err != nil {
		log.Fatal(err)
	}
	return &EndershareDB{db: db}
}

func (db *EndershareDB) GetKeys() *crypto.CryptoKeys {
	rows, err := db.db.Query("SELECT key, value FROM node WHERE key IN ('master_private_key', 'peer_private_key', 'aes_key')")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	keys := make(map[string]string)
	count := 0
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			log.Fatal(err)
		}
		keys[key] = value
		count++
	}

	if count < 3 {
		return nil
	}

	mpriv, err := base64.StdEncoding.DecodeString(keys["master_private_key"])
	if err != nil {
		log.Fatal(err)
	}
	ppriv, err := base64.StdEncoding.DecodeString(keys["peer_private_key"])
	if err != nil {
		log.Fatal(err)
	}
	aesKey, err := base64.StdEncoding.DecodeString(keys["aes_key"])
	if err != nil {
		log.Fatal(err)
	}

	return crypto.NewCryptoKeysFromBytes(mpriv, ppriv, aesKey)
}

func (db *EndershareDB) StoreKeys(keys *crypto.CryptoKeys) {
	masterPrivEnc := base64.StdEncoding.EncodeToString(keys.MasterPrivateKey)
	peerPrivEnc := base64.StdEncoding.EncodeToString(keys.PeerPrivateKey)
	aesKeyEnc := base64.StdEncoding.EncodeToString(keys.AESKey)

	insertStmt := `
	INSERT OR REPLACE INTO node (key, value) VALUES
		('master_private_key', ?),
		('peer_private_key', ?),
		('aes_key', ?);
	`
	_, err := db.db.Exec(insertStmt, masterPrivEnc, peerPrivEnc, aesKeyEnc)
	if err != nil {
		log.Fatal(err)
	}
}
