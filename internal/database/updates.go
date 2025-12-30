package database

import (
	"database/sql"
)

func (db *EndershareDB) InsertSignedUpdate(updateID uint64, signedUpdateJSON string) error {
	query := `INSERT OR REPLACE INTO updates (update_id, signed_update_json) VALUES (?, ?)`
	_, err := db.db.Exec(query, updateID, signedUpdateJSON)
	return err
}

func (db *EndershareDB) GetLatestUpdate() (string, error) {
	query := `SELECT signed_update_json FROM updates ORDER BY update_id DESC LIMIT 1`
	row := db.db.QueryRow(query)

	var signedUpdateJSON string
	err := row.Scan(&signedUpdateJSON)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return signedUpdateJSON, nil
}

func (db *EndershareDB) GetUpdateByID(updateID uint64) (string, error) {
	query := `SELECT signed_update_json FROM updates WHERE update_id = ?`
	row := db.db.QueryRow(query)

	var signedUpdateJSON string
	err := row.Scan(&signedUpdateJSON)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return signedUpdateJSON, nil
}
