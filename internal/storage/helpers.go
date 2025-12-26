package storage

import (
	"encoding/json"
	"io"
	"os"

	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"lukechampine.com/blake3"
)

// streamEncryptFileWithHash encrypts a file and returns the hash of the encrypted content
func streamEncryptFileWithHash(srcPath, destPath string, key []byte) ([]byte, error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return nil, err
	}
	defer destFile.Close()

	hasher := blake3.New(32, nil)
	if err := crypto.EncryptStream(destFile, srcFile, key, hasher); err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}

// streamDecryptFile decrypts a file from source to destination
func streamDecryptFile(srcPath, destPath string, key []byte) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	return crypto.DecryptStream(destFile, srcFile, key)
}

// loadNextFolderID scans the database to find the maximum folder ID
func loadNextFolderID(db *database.EndershareDB, aesKey []byte) int {
	rows, err := db.GetAllData()
	if err != nil {
		return 0
	}

	maxFolderID := -1
	for _, entry := range rows {
		decryptedKey, err := crypto.Decrypt(entry.Key, aesKey)
		if err != nil {
			continue
		}

		var folder FolderEntry
		if err := json.Unmarshal(decryptedKey, &folder); err != nil {
			continue
		}

		if folder.Type == TypeFolder && folder.FolderID > maxFolderID {
			maxFolderID = folder.FolderID
		}
	}

	return maxFolderID + 1
}

// getOriginalFileSize returns the size of a file before encryption
func getOriginalFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// hexEncode converts bytes to hex string for filename
func hexEncode(data []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0xf]
	}
	return string(result)
}

// copyFile is a helper for creating temporary files
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
