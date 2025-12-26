package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
)

type Storage struct {
	db           *database.EndershareDB
	aesKey       []byte
	dataDir      string
	nextFolderID int
}

// NewStorage creates a new storage instance
func NewStorage(db *database.EndershareDB, aesKey []byte) *Storage {
	dataDir := "./data"
	os.MkdirAll(dataDir, 0755)

	s := &Storage{
		db:           db,
		aesKey:       aesKey,
		dataDir:      dataDir,
		nextFolderID: loadNextFolderID(db, aesKey),
	}

	return s
}

// AddFile adds a file from local filesystem to encrypted storage
func (s *Storage) AddFile(localPath string, name string, folderID int) error {
	size, err := getOriginalFileSize(localPath)
	if err != nil {
		return err
	}

	tempFile := filepath.Join(s.dataDir, "temp_"+name)
	fileHash, err := streamEncryptFileWithHash(localPath, tempFile, s.aesKey)
	if err != nil {
		return err
	}

	finalPath := filepath.Join(s.dataDir, hexEncode(fileHash))
	if err := os.Rename(tempFile, finalPath); err != nil {
		os.Remove(tempFile)
		return err
	}

	now := time.Now()
	fileEntry := FileEntry{
		Type:       TypeFile,
		Name:       name,
		CreatedAt:  now,
		ModifiedAt: now,
		Size:       size,
		FolderID:   folderID,
	}

	keyJSON, err := json.Marshal(fileEntry)
	if err != nil {
		return err
	}

	encryptedKey, err := crypto.Encrypt(keyJSON, s.aesKey)
	if err != nil {
		return err
	}

	hash := crypto.ComputeDataHash(append(encryptedKey, fileHash...))

	return s.db.PutData(encryptedKey, fileHash, hash)
}

// GetFile exports a file from encrypted storage to local filesystem
func (s *Storage) GetFile(name string, folderID int, destPath string) error {
	entries, err := s.db.GetAllData()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		decryptedKey, err := crypto.Decrypt(entry.Key, s.aesKey)
		if err != nil {
			continue
		}

		var fileEntry FileEntry
		if err := json.Unmarshal(decryptedKey, &fileEntry); err != nil {
			continue
		}

		if fileEntry.Type == TypeFile && fileEntry.Name == name && fileEntry.FolderID == folderID {
			srcPath := filepath.Join(s.dataDir, hexEncode(entry.Value))
			return streamDecryptFile(srcPath, destPath, s.aesKey)
		}
	}

	return fmt.Errorf("file not found: %s in folder %d", name, folderID)
}

// CreateFolder creates a new folder
func (s *Storage) CreateFolder(name string, parentFolderID int) (int, error) {
	folderID := s.nextFolderID
	s.nextFolderID++

	folderEntry := FolderEntry{
		Type:           TypeFolder,
		FolderID:       folderID,
		Name:           name,
		ParentFolderID: parentFolderID,
	}

	keyJSON, err := json.Marshal(folderEntry)
	if err != nil {
		return 0, err
	}

	encryptedKey, err := crypto.Encrypt(keyJSON, s.aesKey)
	if err != nil {
		return 0, err
	}

	hash := crypto.ComputeDataHash(encryptedKey)

	if err := s.db.PutData(encryptedKey, nil, hash); err != nil {
		return 0, err
	}

	return folderID, nil
}

// DeleteFile removes a file from storage
func (s *Storage) DeleteFile(name string, folderID int) error {
	entries, err := s.db.GetAllData()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		decryptedKey, err := crypto.Decrypt(entry.Key, s.aesKey)
		if err != nil {
			continue
		}

		var fileEntry FileEntry
		if err := json.Unmarshal(decryptedKey, &fileEntry); err != nil {
			continue
		}

		if fileEntry.Type == TypeFile && fileEntry.Name == name && fileEntry.FolderID == folderID {
			return s.db.DeleteData(entry.Key)
		}
	}

	return fmt.Errorf("file not found: %s in folder %d", name, folderID)
}

// DeleteFolder removes a folder
func (s *Storage) DeleteFolder(folderID int) error {
	entries, err := s.db.GetAllData()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		decryptedKey, err := crypto.Decrypt(entry.Key, s.aesKey)
		if err != nil {
			continue
		}

		var folderEntry FolderEntry
		if err := json.Unmarshal(decryptedKey, &folderEntry); err != nil {
			continue
		}

		if folderEntry.Type == TypeFolder && folderEntry.FolderID == folderID {
			return s.db.DeleteData(entry.Key)
		}
	}

	return fmt.Errorf("folder not found: %d", folderID)
}

// ListFolder lists files and folders in a folder
func (s *Storage) ListFolder(folderID int) ([]interface{}, error) {
	entries, err := s.db.GetAllData()
	if err != nil {
		return nil, err
	}

	var results []interface{}

	for _, entry := range entries {
		decryptedKey, err := crypto.Decrypt(entry.Key, s.aesKey)
		if err != nil {
			continue
		}

		var fileEntry FileEntry
		if err := json.Unmarshal(decryptedKey, &fileEntry); err == nil {
			if fileEntry.Type == TypeFile && fileEntry.FolderID == folderID {
				results = append(results, fileEntry)
				continue
			}
		}

		var folderEntry FolderEntry
		if err := json.Unmarshal(decryptedKey, &folderEntry); err == nil {
			if folderEntry.Type == TypeFolder && folderEntry.ParentFolderID == folderID {
				results = append(results, folderEntry)
			}
		}
	}

	return results, nil
}
