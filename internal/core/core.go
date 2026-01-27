package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"github.com/notassigned/endershare/internal/p2p"
	"github.com/notassigned/endershare/internal/storage"
)

type Core struct {
	p2pNode       *p2p.P2PNode
	keys          *crypto.CryptoKeys
	db            *database.EndershareDB
	storage       *storage.Storage
	merkleTree    *crypto.MerkleTree
	publishUpdate func([]byte) error
	OnDataUpdated func() // Called when data is synced from another device
}

func coreStartup(initMode bool) *Core {
	core := &Core{
		db: database.Create(),
	}

	//Check for keys in db
	keys := core.db.GetKeys()
	if keys == nil {
		if initMode {
			// Master node initialization - generate full keys
			var mnemonic string
			keys, mnemonic = crypto.CreateCryptoKeys()
			core.db.StoreKeys(keys)
			fmt.Println("Generated new keys with mnemonic:", mnemonic)
		} else {
			// Replica node - generate peer-only keys
			keys = crypto.CreatePeerOnlyKeys()
			core.db.StoreKeys(keys)
			fmt.Println("Generated peer keys (waiting for network binding)")
		}
	}

	ctx := context.Background()
	p2pNode, err := p2p.NewP2PNode(keys.PeerPrivateKey, ctx, core.db.GetPeers(), 13000)
	if err != nil {
		panic(fmt.Sprintf("Error starting P2P node: %v", err))
	}

	core.p2pNode = p2pNode
	core.keys = keys
	// Storage might not have AES key yet for replica nodes - will be set after binding
	if keys.AESKey != nil {
		core.storage = storage.NewStorage(core.db, keys.AESKey)
	}

	// Initialize node table properties if not set
	core.initializeNodeProperties()

	// Build merkle tree from data table
	dataHashes := core.db.GetAllDataHashes()
	core.merkleTree = crypto.NewMerkleTree(dataHashes)

	// Store merkle root in node properties
	rootHash := core.merkleTree.GetRootHash()
	core.db.SetNodeProperty("data_hash", base64.StdEncoding.EncodeToString(rootHash))

	// Setup sync stream handlers
	core.setupSyncHandlers()

	return core
}

// initializeNodeProperties initializes node table properties if they don't exist
func (c *Core) initializeNodeProperties() {
	// Initialize current_update_id to 0 if not set
	if _, err := c.db.GetNodeProperty("current_update_id"); err != nil {
		c.db.SetNodeProperty("current_update_id", "0")
	}

	// Initialize peer_list_hash to zero hash if not set
	if _, err := c.db.GetNodeProperty("peer_list_hash"); err != nil {
		zeroHash := make([]byte, 32)
		c.db.SetNodeProperty("peer_list_hash", base64.StdEncoding.EncodeToString(zeroHash))
	}

	// Initialize data_hash to zero hash if not set
	if _, err := c.db.GetNodeProperty("data_hash"); err != nil {
		zeroHash := make([]byte, 32)
		c.db.SetNodeProperty("data_hash", base64.StdEncoding.EncodeToString(zeroHash))
	}
}

// setupSyncHandlers registers stream handlers for syncing
func (c *Core) setupSyncHandlers() {
	c.p2pNode.NewStreamHandler("/endershare/peer-list/1.0", c.handlePeerListRequest)
	c.p2pNode.NewStreamHandler("/endershare/tree-bucket-hashes/1.0", c.handleTreeBucketHashesRequest)
	c.p2pNode.NewStreamHandler("/endershare/data-bucket-hashes/1.0", c.handleDataBucketHashesRequest)
	c.p2pNode.NewStreamHandler("/endershare/metadata/1.0", c.handleMetadataRequest)
	c.p2pNode.NewStreamHandler("/endershare/file-data/1.0", c.handleFileDataRequest)
}

// NewCore creates and initializes a Core instance for use with the UI.
// It starts the P2P node and background sync but does not block.
func NewCore() (*Core, error) {
	c := coreStartup(true)

	// Setup notify service
	err := c.setupNotifyService(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to setup notify service: %w", err)
	}

	// Start connection management in background
	if c.keys.MasterPublicKey != nil {
		go c.p2pNode.ManageConnections(context.Background(), string(c.keys.MasterPublicKey))
	}

	// Start periodic sync in background
	go func() {
		c.RequestLatestUpdate()
		t := time.NewTicker(time.Second * 15)
		for range t.C {
			c.RequestLatestUpdate()
		}
	}()

	return c, nil
}

// Storage returns the storage instance for file operations
func (c *Core) Storage() *storage.Storage {
	return c.storage
}

// DB returns the database instance
func (c *Core) DB() *database.EndershareDB {
	return c.db
}

// IsMaster returns true if this node has the master private key
func (c *Core) IsMaster() bool {
	return c.keys.MasterPrivateKey != nil
}

// NewCoreForBinding creates a Core instance for replica binding (no master keys yet)
func NewCoreForBinding(db *database.EndershareDB, keys *crypto.CryptoKeys) (*Core, error) {
	ctx := context.Background()
	p2pNode, err := p2p.NewP2PNode(keys.PeerPrivateKey, ctx, db.GetPeers(), 13000)
	if err != nil {
		return nil, fmt.Errorf("error starting P2P node: %w", err)
	}

	c := &Core{
		db:      db,
		p2pNode: p2pNode,
		keys:    keys,
	}

	c.initializeNodeProperties()
	return c, nil
}

// StartBinding starts the binding process for a replica node
// Returns the 4-word sync phrase and calls onComplete when binding finishes
func (c *Core) StartBinding(ctx context.Context, onComplete func(info *p2p.ClientInfo)) (string, error) {
	clientInfo, phrase, err := p2p.StartBindingService(c.p2pNode, ctx)
	if err != nil {
		return "", err
	}

	// Wait for binding to complete in background
	go func() {
		select {
		case info := <-clientInfo:
			if info != nil && onComplete != nil {
				onComplete(info)
			}
		case <-ctx.Done():
			// Cancelled
		}
	}()

	return phrase, nil
}

// GetPeerStatus returns whether a peer is currently connected and when it was last seen
func (c *Core) GetPeerStatus(peerID string) (isOnline bool, lastSeen time.Time) {
	if c.p2pNode == nil {
		return false, time.Time{}
	}
	return c.p2pNode.GetPeerStatus(peerID)
}

// ReplacePeers updates the P2P node's in-memory peer map
func (c *Core) ReplacePeers(peers []peer.AddrInfo) {
	c.p2pNode.ReplacePeers(peers)
}
