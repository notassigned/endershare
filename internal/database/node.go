package database

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"strconv"
)

func (db *EndershareDB) getNodeProperty(key string) (string, error) {
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

func (db *EndershareDB) setNodeProperty(key string, value string) error {
	_, err := db.db.Exec("INSERT OR REPLACE INTO node (key, value) VALUES (?, ?)", key, value)
	return err
}

func (db *EndershareDB) DeleteNodeProperty(key string) error {
	_, err := db.db.Exec("DELETE FROM node WHERE key = ?", key)
	return err
}

// Typed getters/setters for node properties

func (db *EndershareDB) GetCurrentUpdateID() (uint64, error) {
	s, err := db.getNodeProperty("current_update_id")
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, 64)
}

func (db *EndershareDB) SetCurrentUpdateID(id uint64) error {
	return db.setNodeProperty("current_update_id", fmt.Sprintf("%d", id))
}

func (db *EndershareDB) GetDataRootHash() ([]byte, error) {
	s, err := db.getNodeProperty("data_hash")
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(s)
}

func (db *EndershareDB) SetDataRootHash(hash []byte) error {
	return db.setNodeProperty("data_hash", base64.StdEncoding.EncodeToString(hash))
}

func (db *EndershareDB) GetPeerListHash() ([]byte, error) {
	s, err := db.getNodeProperty("peer_list_hash")
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(s)
}

func (db *EndershareDB) SetPeerListHash(hash []byte) error {
	return db.setNodeProperty("peer_list_hash", base64.StdEncoding.EncodeToString(hash))
}

func (db *EndershareDB) GetLatestUpdateJSON() (string, error) {
	return db.getNodeProperty("latest_update")
}

func (db *EndershareDB) SetLatestUpdateJSON(jsonStr string) error {
	return db.setNodeProperty("latest_update", jsonStr)
}

func (db *EndershareDB) SetMasterPublicKey(key []byte) error {
	return db.setNodeProperty("master_public_key", base64.StdEncoding.EncodeToString(key))
}
