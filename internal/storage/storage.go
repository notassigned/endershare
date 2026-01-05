package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"lukechampine.com/blake3"
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

	hash := crypto.ComputeDataHash(encryptedKey, fileHash, size)

	return s.db.PutData(encryptedKey, fileHash, size, hash)
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

	hash := crypto.ComputeDataHash(encryptedKey, nil, 0)

	if err := s.db.PutData(encryptedKey, nil, 0, hash); err != nil {
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

// FileExists checks if a file exists in storage by its hash
func (s *Storage) FileExists(fileHash []byte) bool {
	filePath := filepath.Join(s.dataDir, hexEncode(fileHash))
	_, err := os.Stat(filePath)
	return err == nil
}

// OpenFileForReading opens a file for reading and returns the file handle and total size
func (s *Storage) OpenFileForReading(fileHash []byte) (*os.File, int64, error) {
	filePath := filepath.Join(s.dataDir, hexEncode(fileHash))
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, stat.Size(), nil
}

// AppendFileData appends data to a file (for resumable downloads)
func (s *Storage) AppendFileData(fileHash []byte, data []byte) error {
	filePath := filepath.Join(s.dataDir, hexEncode(fileHash))

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// VerifyFile verifies the hash of a stored file and removes it if invalid
func (s *Storage) VerifyFile(fileHash []byte) error {
	f, _, err := s.OpenFileForReading(fileHash)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := blake3.New(32, nil)
	buf := make([]byte, 64*1024) // 64KB buffer

	for {
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		hasher.Write(buf[:n])
	}

	computedHash := hasher.Sum(nil)
	if !bytes.Equal(computedHash, fileHash) {
		f.Close()
		os.Remove(f.Name())
		return fmt.Errorf("file hash verification failed")
	}

	return nil
}
