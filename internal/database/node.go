package database

import "database/sql"

func (db *EndershareDB) GetNodeProperty(key string) (string, error) {
	rows, err := db.db.Query("SELECT value FROM node WHERE key = ?", key)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var value string
	if rows.Next() {
		if err := rows.Scan(&value); err != nil {
			return "", err
		}
		return value, nil
	}
	return "", sql.ErrNoRows
}

func (db *EndershareDB) SetNodeProperty(key string, value string) error {
	_, err := db.db.Exec("INSERT OR REPLACE INTO node (key, value) VALUES (?, ?)", key, value)
	return err
}

func (db *EndershareDB) DeleteNodeProperty(key string) error {
	_, err := db.db.Exec("DELETE FROM node WHERE key = ?", key)
	return err
}
