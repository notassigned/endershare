package database

import (
	"database/sql"

	"github.com/notassigned/endershare/internal/crypto"
	"lukechampine.com/blake3"
)

type DataEntry struct {
	Key   []byte
	Value []byte
	Hash  []byte
}

func (db *EndershareDB) PutData(key []byte, value []byte) error {
	hash := crypto.ComputeDataHash(append(key, value...))
	_, err := db.db.Exec("INSERT OR REPLACE INTO data (key, value, hash) VALUES (?, ?, ?)", key, value, hash)
	return err
}

func (db *EndershareDB) GetData(key []byte) ([]byte, error) {
	rows, err := db.db.Query("SELECT value FROM data WHERE key = ?", key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var value []byte
	if rows.Next() {
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		return value, nil
	}
	return nil, sql.ErrNoRows
}

func (db *EndershareDB) DeleteData(key []byte) error {
	_, err := db.db.Exec("DELETE FROM data WHERE key = ?", key)
	return err
}

func (db *EndershareDB) GetDataHash() ([]byte, error) {
	rows, err := db.db.Query("SELECT hash FROM data ORDER BY hash")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	h := blake3.New(32, nil)
	h.Write([]byte{1})

	for rows.Next() {
		var hash []byte
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		h.Write(hash)
	}
	return h.Sum(nil), nil
}
