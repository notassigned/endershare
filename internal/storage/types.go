package storage

import "time"

type EntryType string

const (
	TypeFile   EntryType = "file"
	TypeFolder EntryType = "folder"
)

type FileEntry struct {
	Type       EntryType `json:"type"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"createdAt"`
	ModifiedAt time.Time `json:"modifiedAt"`
	Size       int64     `json:"size"`
	FolderID   int       `json:"folderId"`
}

type FolderEntry struct {
	Type           EntryType `json:"type"`
	FolderID       int       `json:"folderId"`
	Name           string    `json:"name"`
	ParentFolderID int       `json:"parentFolderId"`
}
