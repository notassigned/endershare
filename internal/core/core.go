package core

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"github.com/notassigned/endershare/internal/p2p"
	"github.com/notassigned/endershare/internal/storage"
)

type Core struct {
	p2pNode *p2p.P2PNode
	keys    *crypto.CryptoKeys
	db      *database.EndershareDB
	storage *storage.Storage
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
	p2pNode, err := p2p.NewP2PNode(keys.PeerPrivateKey, ctx, core.db.GetPeers())
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

	// Setup sync stream handlers
	core.p2pNode.SetupSyncHandlers(core.db)

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
