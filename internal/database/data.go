package database

import (
	"bytes"
	"database/sql"
	"math/big"

	"lukechampine.com/blake3"
)

type DataEntry struct {
	Key   []byte
	Value []byte
	Size  int64
	Hash  []byte
}

func (db *EndershareDB) PutData(key []byte, value []byte, size int64, hash []byte) error {
	_, err := db.db.Exec("INSERT OR REPLACE INTO data (key, value, size, hash) VALUES (?, ?, ?, ?)", key, value, size, hash)
	return err
}

func (db *EndershareDB) PutDataWithTag(key []byte, value []byte, size int64, hash []byte, folderTag []byte) error {
	_, err := db.db.Exec("INSERT OR REPLACE INTO data (key, value, size, hash, folder_tag) VALUES (?, ?, ?, ?, ?)", key, value, size, hash, folderTag)
	return err
}

func (db *EndershareDB) SetFolderTag(key []byte, folderTag []byte) error {
	_, err := db.db.Exec("UPDATE data SET folder_tag = ? WHERE key = ?", folderTag, key)
	return err
}

// GetDataByFolderTag returns entries matching a folder tag for fast folder listing
func (db *EndershareDB) GetDataByFolderTag(folderTag []byte) ([]DataEntry, error) {
	rows, err := db.db.Query("SELECT key, value, size, hash FROM data WHERE folder_tag = ?", folderTag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DataEntry
	for rows.Next() {
		var entry DataEntry
		if err := rows.Scan(&entry.Key, &entry.Value, &entry.Size, &entry.Hash); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// GetDataWithNullFolderTag returns entries that don't have a folder_tag set yet
func (db *EndershareDB) GetDataWithNullFolderTag() ([]DataEntry, error) {
	rows, err := db.db.Query("SELECT key, value, size, hash FROM data WHERE folder_tag IS NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DataEntry
	for rows.Next() {
		var entry DataEntry
		if err := rows.Scan(&entry.Key, &entry.Value, &entry.Size, &entry.Hash); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
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

func (db *EndershareDB) GetAllData() ([]DataEntry, error) {
	rows, err := db.db.Query("SELECT key, value, size, hash FROM data")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DataEntry
	for rows.Next() {
		var entry DataEntry
		if err := rows.Scan(&entry.Key, &entry.Value, &entry.Size, &entry.Hash); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
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

// GetAllDataHashes returns all hash column values for merkle tree construction
func (db *EndershareDB) GetAllDataHashes() [][]byte {
	rows, err := db.db.Query("SELECT hash FROM data ORDER BY hash")
	if err != nil {
		return [][]byte{}
	}
	defer rows.Close()

	var hashes [][]byte
	for rows.Next() {
		var hash []byte
		if err := rows.Scan(&hash); err != nil {
			continue
		}
		hashes = append(hashes, hash)
	}
	return hashes
}

// GetBucketHashes returns data entry hashes for a specific bucket index
func (db *EndershareDB) GetBucketHashes(bucketIdx int, numBuckets int) [][]byte {
	start, end := computeBucketRange(bucketIdx, numBuckets)

	rows, err := db.db.Query("SELECT hash FROM data WHERE hash >= ? AND hash < ? ORDER BY hash", start, end)
	if err != nil {
		return [][]byte{}
	}
	defer rows.Close()

	var hashes [][]byte
	for rows.Next() {
		var hash []byte
		if err := rows.Scan(&hash); err != nil {
			continue
		}
		hashes = append(hashes, hash)
	}
	return hashes
}

// GetDataByHashes returns complete entries for specific hashes
func (db *EndershareDB) GetDataByHashes(hashes [][]byte) []DataEntry {
	if len(hashes) == 0 {
		return []DataEntry{}
	}

	var entries []DataEntry
	for _, hash := range hashes {
		rows, err := db.db.Query("SELECT key, value, size, hash FROM data WHERE hash = ?", hash)
		if err != nil {
			continue
		}
		defer rows.Close()

		if rows.Next() {
			var entry DataEntry
			if err := rows.Scan(&entry.Key, &entry.Value, &entry.Size, &entry.Hash); err == nil {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

// MarkAllStale marks all entries as stale before sync
func (db *EndershareDB) MarkAllStale() error {
	_, err := db.db.Exec("UPDATE data SET in_current = 0")
	return err
}

// MarkHashCurrent marks a specific hash as current (in_current = 1)
func (db *EndershareDB) MarkHashCurrent(hash []byte) error {
	_, err := db.db.Exec("UPDATE data SET in_current = 1 WHERE hash = ?", hash)
	return err
}

// GetStaleHashes returns hashes of all stale entries (in_current = 0)
func (db *EndershareDB) GetStaleHashes() [][]byte {
	rows, err := db.db.Query("SELECT hash FROM data WHERE in_current = 0")
	if err != nil {
		return [][]byte{}
	}
	defer rows.Close()

	var hashes [][]byte
	for rows.Next() {
		var hash []byte
		if err := rows.Scan(&hash); err != nil {
			continue
		}
		hashes = append(hashes, hash)
	}
	return hashes
}

// DeleteStaleEntries deletes all stale entries (in_current = 0)
func (db *EndershareDB) DeleteStaleEntries() error {
	_, err := db.db.Exec("DELETE FROM data WHERE in_current = 0")
	return err
}

// SetDownloadProgress updates download progress for a file
func (db *EndershareDB) SetDownloadProgress(hash []byte, progress int64) error {
	_, err := db.db.Exec("UPDATE data SET download_progress = ? WHERE value = ?", progress, hash)
	return err
}

// GetDownloadProgress returns download progress for a file (0 if not started, size of the file if complete)
func (db *EndershareDB) GetDownloadProgress(hash []byte) int64 {
	rows, err := db.db.Query("SELECT download_progress FROM data WHERE value = ?", hash)
	if err != nil {
		return 0
	}
	defer rows.Close()

	if rows.Next() {
		var progress int64
		if err := rows.Scan(&progress); err != nil {
			return 0
		}
		return progress
	}
	return 0
}

// GetStorageStats returns total entry count and total size in bytes
func (db *EndershareDB) GetStorageStats() (int64, int64) {
	var count, totalSize int64
	row := db.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(size), 0) FROM data WHERE in_current = 1")
	if err := row.Scan(&count, &totalSize); err != nil {
		return 0, 0
	}
	return count, totalSize
}

// computeBucketRange calculates the hash range for a bucket index
// This matches the logic in merkletree.go:getBucketIndex()
func computeBucketRange(bucketIdx int, numBuckets int) ([]byte, []byte) {
	if numBuckets <= 1 {
		// Single bucket covers entire hash space
		start := make([]byte, 32)
		end := bytes.Repeat([]byte{0xFF}, 32)
		return start, end
	}

	// Calculate bucket size: 2^256 / numBuckets
	maxHash := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256
	bucketSize := new(big.Int).Div(maxHash, big.NewInt(int64(numBuckets)))

	// Calculate start: bucketIdx * bucketSize
	startInt := new(big.Int).Mul(big.NewInt(int64(bucketIdx)), bucketSize)

	// Calculate end: (bucketIdx + 1) * bucketSize
	endInt := new(big.Int).Mul(big.NewInt(int64(bucketIdx+1)), bucketSize)

	// Handle last bucket to cover remainder
	if bucketIdx == numBuckets-1 {
		endInt = new(big.Int).Set(maxHash)
	}

	// Convert to byte slices (pad to 32 bytes)
	start := make([]byte, 32)
	end := make([]byte, 32)

	startBytes := startInt.Bytes()
	endBytes := endInt.Bytes()

	copy(start[32-len(startBytes):], startBytes)
	copy(end[32-len(endBytes):], endBytes)

	return start, end
}
