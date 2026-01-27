package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/notassigned/endershare/internal/core"
	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"github.com/notassigned/endershare/internal/p2p"
	"github.com/notassigned/endershare/internal/storage"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// FolderItem represents a file or folder for the frontend
type FolderItem struct {
	Type       string `json:"type"` // "file" or "folder"
	Name       string `json:"name"`
	FolderID   int    `json:"folderId"`   // For folders only
	Size       int64  `json:"size"`       // For files only
	ModifiedAt string `json:"modifiedAt"` // ISO format for files
}

// PathSegment represents a breadcrumb segment
type PathSegment struct {
	Name     string `json:"name"`
	FolderID int    `json:"folderId"`
}

// PeerInfo represents a peer device for the frontend
type PeerInfo struct {
	PeerID   string `json:"peerId"`
	IsOnline bool   `json:"isOnline"`
	LastSeen string `json:"lastSeen"`
}

// App struct holds application state
type App struct {
	ctx          context.Context
	db           *database.EndershareDB
	core         *core.Core
	keys         *crypto.CryptoKeys
	stor         *storage.Storage
	syncPhrase   string
	bindingMutex sync.Mutex
	bindCancel   context.CancelFunc
}

// NewApp creates a new App instance
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.db = database.Create()
	a.keys = a.db.GetKeys()

	// If we have full keys (including AES), initialize storage and core
	if a.keys != nil && a.keys.AESKey != nil {
		a.initializeCore()
	}
}

// initializeCore sets up the core and storage when keys are available
func (a *App) initializeCore() {
	if a.keys == nil || a.keys.AESKey == nil {
		return
	}
	a.stor = storage.NewStorage(a.db, a.keys.AESKey)

	// Initialize core for background sync
	var err error
	a.core, err = core.NewCore()
	if err != nil {
		fmt.Println("Warning: Failed to initialize core:", err)
	}
}

// GetAppState returns the current application state
// Returns: "fresh", "binding", "locked", "unlocked"
func (a *App) GetAppState() string {
	if a.keys == nil {
		return "fresh"
	}

	// Check if we're in binding mode
	if a.syncPhrase != "" {
		return "binding"
	}

	// Has peer keys but no master public key - need to bind
	if a.keys.MasterPublicKey == nil {
		return "fresh" // Should start binding
	}

	// Has master public key but no AES key - locked (replica or need mnemonic)
	if a.keys.AESKey == nil {
		return "locked"
	}

	return "unlocked"
}

// GetSyncPhrase returns the current binding sync phrase
func (a *App) GetSyncPhrase() string {
	return a.syncPhrase
}

// CreateNewVault generates new master keys and returns the mnemonic
func (a *App) CreateNewVault() (string, error) {
	a.bindingMutex.Lock()
	defer a.bindingMutex.Unlock()

	keys, mnemonic := crypto.CreateCryptoKeys()
	a.db.StoreKeys(keys)
	a.keys = keys
	a.initializeCore()

	return mnemonic, nil
}

// StartReplicaBinding starts the binding process and returns the 4-word phrase
func (a *App) StartReplicaBinding() (string, error) {
	a.bindingMutex.Lock()
	defer a.bindingMutex.Unlock()

	// Generate peer-only keys if we don't have any
	if a.keys == nil {
		a.keys = crypto.CreatePeerOnlyKeys()
		a.db.StoreKeys(a.keys)
	}

	// Start binding in background - this will be handled by the core
	// For now, we need to initialize core first
	var err error
	a.core, err = core.NewCoreForBinding(a.db, a.keys)
	if err != nil {
		return "", fmt.Errorf("failed to initialize for binding: %w", err)
	}

	// Start binding and get the sync phrase
	ctx, cancel := context.WithCancel(context.Background())
	a.bindCancel = cancel

	phrase, err := a.core.StartBinding(ctx, func(info *p2p.ClientInfo) {
		// Store master public key
		a.keys.MasterPublicKey = info.MasterPublicKey
		a.db.StoreKeys(a.keys)

		// Add master node to peers table
		if err := a.db.AddPeer(info.AddrInfo); err != nil {
			fmt.Println("Warning: Failed to add master peer:", err)
		}

		// Add all peers from the peer list
		for _, peerInfo := range info.PeerList {
			if err := a.db.AddPeer(peerInfo); err != nil {
				fmt.Printf("Warning: Failed to add peer %s: %v\n", peerInfo.ID, err)
			}
		}

		// Update P2P node's in-memory peer map
		allPeers := append(info.PeerList, info.AddrInfo)
		a.core.ReplacePeers(allPeers)

		a.syncPhrase = ""
		runtime.EventsEmit(a.ctx, "binding-complete")
	})
	if err != nil {
		return "", err
	}

	a.syncPhrase = phrase
	return phrase, nil
}

// CancelBinding cancels the current binding process
func (a *App) CancelBinding() error {
	a.bindingMutex.Lock()
	defer a.bindingMutex.Unlock()

	if a.bindCancel != nil {
		a.bindCancel()
		a.bindCancel = nil
	}
	a.syncPhrase = ""
	return nil
}

// UnlockWithMnemonic unlocks the vault using the mnemonic
func (a *App) UnlockWithMnemonic(mnemonic string) error {
	a.bindingMutex.Lock()
	defer a.bindingMutex.Unlock()

	keys := crypto.SetupKeysFromMnemonic(mnemonic)

	// Verify the mnemonic matches our stored master public key if we have one
	if a.keys != nil && a.keys.MasterPublicKey != nil {
		if string(keys.MasterPublicKey) != string(a.keys.MasterPublicKey) {
			return fmt.Errorf("mnemonic does not match this vault")
		}
		// Keep our peer keys
		keys.PeerPrivateKey = a.keys.PeerPrivateKey
		keys.PeerPublicKey = a.keys.PeerPublicKey
	}

	a.db.StoreKeys(keys)
	a.keys = keys
	a.initializeCore()

	return nil
}

// ListFolder returns files and folders in the specified folder
func (a *App) ListFolder(folderID int) ([]FolderItem, error) {
	if a.stor == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	items, err := a.stor.ListFolder(folderID)
	if err != nil {
		return nil, err
	}

	result := make([]FolderItem, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case storage.FileEntry:
			result = append(result, FolderItem{
				Type:       "file",
				Name:       v.Name,
				Size:       v.Size,
				ModifiedAt: v.ModifiedAt.Format(time.RFC3339),
			})
		case storage.FolderEntry:
			result = append(result, FolderItem{
				Type:     "folder",
				Name:     v.Name,
				FolderID: v.FolderID,
			})
		}
	}

	return result, nil
}

// CreateFolder creates a new folder and returns its ID
func (a *App) CreateFolder(name string, parentID int) (int, error) {
	if a.stor == nil {
		return 0, fmt.Errorf("vault is locked")
	}

	folderID, entry, err := a.stor.CreateFolderWithEntry(name, parentID)
	if err != nil {
		return 0, err
	}

	// Publish update if master
	if a.core != nil && a.core.IsMaster() {
		if err := a.core.PublishDataUpdate("ADD", entry.Key, entry.Value, entry.Size, entry.Hash); err != nil {
			fmt.Println("Warning: Failed to publish data update:", err)
		}
	}

	return folderID, nil
}

// AddFile opens a file picker and adds the selected file to the folder
func (a *App) AddFile(folderID int) error {
	if a.stor == nil {
		return fmt.Errorf("vault is locked")
	}

	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File to Add",
	})
	if err != nil {
		return err
	}
	if filePath == "" {
		return nil // User cancelled
	}

	fileName := filepath.Base(filePath)
	entry, err := a.stor.AddFileWithEntry(filePath, fileName, folderID)
	if err != nil {
		return err
	}

	// Publish update if master
	if a.core != nil && a.core.IsMaster() {
		if err := a.core.PublishDataUpdate("ADD", entry.Key, entry.Value, entry.Size, entry.Hash); err != nil {
			fmt.Println("Warning: Failed to publish data update:", err)
		}
	}

	return nil
}

// ExportFile exports a file to the local filesystem
func (a *App) ExportFile(name string, folderID int) error {
	if a.stor == nil {
		return fmt.Errorf("vault is locked")
	}

	destPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export File",
		DefaultFilename: name,
	})
	if err != nil {
		return err
	}
	if destPath == "" {
		return nil // User cancelled
	}

	return a.stor.GetFile(name, folderID, destPath)
}

// DeleteFile removes a file from storage
func (a *App) DeleteFile(name string, folderID int) error {
	if a.stor == nil {
		return fmt.Errorf("vault is locked")
	}

	entry, err := a.stor.DeleteFileWithEntry(name, folderID)
	if err != nil {
		return err
	}

	// Publish update if master
	if a.core != nil && a.core.IsMaster() {
		if err := a.core.PublishDataUpdate("DELETE", entry.Key, entry.Value, entry.Size, entry.Hash); err != nil {
			fmt.Println("Warning: Failed to publish data update:", err)
		}
	}

	return nil
}

// DeleteFolder removes a folder from storage
func (a *App) DeleteFolder(folderID int) error {
	if a.stor == nil {
		return fmt.Errorf("vault is locked")
	}

	entry, err := a.stor.DeleteFolderWithEntry(folderID)
	if err != nil {
		return err
	}

	// Publish update if master
	if a.core != nil && a.core.IsMaster() {
		if err := a.core.PublishDataUpdate("DELETE", entry.Key, entry.Value, entry.Size, entry.Hash); err != nil {
			fmt.Println("Warning: Failed to publish data update:", err)
		}
	}

	return nil
}

// GetFolderPath returns the path segments for breadcrumb navigation
func (a *App) GetFolderPath(folderID int) ([]PathSegment, error) {
	if a.stor == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	if folderID == 0 {
		return []PathSegment{{Name: "/", FolderID: 0}}, nil
	}

	// Build path by traversing parent folders
	path := []PathSegment{}
	currentID := folderID

	for currentID != 0 {
		folder, err := a.getFolderByID(currentID)
		if err != nil {
			break
		}
		path = append([]PathSegment{{Name: folder.Name, FolderID: folder.FolderID}}, path...)
		currentID = folder.ParentFolderID
	}

	// Add root at the beginning
	path = append([]PathSegment{{Name: "/", FolderID: 0}}, path...)

	return path, nil
}

// getFolderByID finds a folder by its ID
func (a *App) getFolderByID(folderID int) (*storage.FolderEntry, error) {
	items, err := a.db.GetAllData()
	if err != nil {
		return nil, err
	}

	for _, entry := range items {
		decrypted, err := crypto.Decrypt(entry.Key, a.keys.AESKey)
		if err != nil {
			continue
		}

		var folder storage.FolderEntry
		if err := parseJSON(decrypted, &folder); err != nil {
			continue
		}

		if folder.Type == storage.TypeFolder && folder.FolderID == folderID {
			return &folder, nil
		}
	}

	return nil, fmt.Errorf("folder not found: %d", folderID)
}

// GetPeers returns all connected peers with their status
func (a *App) GetPeers() ([]PeerInfo, error) {
	peerIDs := a.db.GetAllPeerIDs()
	result := make([]PeerInfo, 0, len(peerIDs))

	for _, peerID := range peerIDs {
		info := PeerInfo{
			PeerID:   truncatePeerID(peerID),
			IsOnline: false,
			LastSeen: "Unknown",
		}

		// Check if peer is connected via core
		if a.core != nil {
			isOnline, lastSeen := a.core.GetPeerStatus(peerID)
			info.IsOnline = isOnline
			if !lastSeen.IsZero() {
				info.LastSeen = formatLastSeen(lastSeen)
			}
		}

		result = append(result, info)
	}

	return result, nil
}

// RemovePeer removes a peer from the network
func (a *App) RemovePeer(peerID string) error {
	return a.db.RemovePeer(peerID)
}

// BindPeerWithPhrase binds a new peer using their 4-word phrase (master only)
func (a *App) BindPeerWithPhrase(phrase string) error {
	if a.core == nil {
		return fmt.Errorf("core not initialized")
	}
	if a.keys == nil || a.keys.MasterPrivateKey == nil {
		return fmt.Errorf("only master nodes can bind new peers")
	}

	return a.core.BindNewPeer(phrase)
}

// IsMaster returns true if this is a master node
func (a *App) IsMaster() bool {
	return a.keys != nil && a.keys.MasterPrivateKey != nil
}

// Helper functions

func truncatePeerID(peerID string) string {
	if len(peerID) > 12 {
		return peerID[:6] + "..." + peerID[len(peerID)-6:]
	}
	return peerID
}

func formatLastSeen(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "Just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
