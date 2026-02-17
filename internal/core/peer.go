package core

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"github.com/notassigned/endershare/internal/p2p"
	"github.com/notassigned/endershare/internal/storage"
)

// getMasterPubKey retrieves the master public key from the database
func getMasterPubKey(db *database.EndershareDB) ed25519.PublicKey {
	k, err := db.GetMasterPubKey()
	if err != nil {
		return nil
	}
	return k
}

// PeerMain (CLI only) is the unified entry point for all nodes (both master and replica)
func PeerMain(initMode bool) {
	var c *Core

	if initMode {
		// Master node initialization
		fmt.Print("Initialize from existing mnemonic? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "y" || input == "yes" {
			fmt.Print("Enter mnemonic: ")
			mnemonicInput, _ := reader.ReadString('\n')
			mnemonic := strings.TrimSpace(mnemonicInput)

			c = coreStartupWithMnemonic(mnemonic)
		} else {
			c = coreStartup(true)
		}

		fmt.Println("Master node initialized successfully")
	} else {
		// Replica node
		c = coreStartup(false)

		// Check if we need to enter binding mode
		masterPubKey := getMasterPubKey(c.db)
		if masterPubKey == nil {
			fmt.Println("Entering binding mode (no master key found)")
			c.bindToMaster()
		}
	}

	// Setup notify service for all nodes
	err := c.setupNotifyService(context.Background())
	if err != nil {
		fmt.Println("Error setting up notify service:", err)
	}

	// Start connection management
	if c.keys.MasterPublicKey != nil {
		go c.p2pNode.ManageConnections(context.Background(), string(c.keys.MasterPublicKey))
	} else {
		fmt.Println("Warning: No master public key available, cannot manage connections yet")
	}

	// Wait indefinitely, periodically requesting latest updates
	t := time.NewTicker(time.Second * 15)
	for {
		c.RequestLatestUpdate()
		<-t.C
	}
}

// BindMain (CLI only) is called by a master node to authorize a new replica peer
func BindMain(syncPhrase string) {
	// Load existing core
	c := coreStartup(true) // Must be a master node

	if c.keys.MasterPrivateKey == nil {
		fmt.Println("Error: This node does not have the master private key")
		fmt.Println("Only master nodes can bind new peers")
		os.Exit(1)
	}

	err := c.BindNewPeer(syncPhrase)
	if err != nil {
		fmt.Println("Error binding peer:", err)
		os.Exit(1)
	}

	fmt.Println("Successfully bound new peer")
}

func (c *Core) GetOtherPeerIDs() []string {
	selfID := c.p2pNode.GetPeerId().String()
	allPeerIDs := c.db.GetAllPeerIDs()
	otherPeerIDs := []string{}
	for _, pid := range allPeerIDs {
		if pid != selfID {
			otherPeerIDs = append(otherPeerIDs, pid)
		}
	}
	return otherPeerIDs
}

// BindNewPeer discovers and authorizes a new replica peer using the sync phrase
func (c *Core) BindNewPeer(syncPhrase string) error {
	if c.keys.MasterPrivateKey == nil {
		return fmt.Errorf("only master nodes can bind new peers")
	}

	// Get existing peers to send to the new peer
	existingPeers := c.db.GetPeers()

	// Discover and bind the new peer, sending them the peer list
	peerInfo, err := p2p.BindNewPeer(
		syncPhrase,
		c.p2pNode,
		c.keys.MasterPublicKey,
		c.keys.MasterPrivateKey,
		existingPeers,
	)
	if err != nil {
		return err
	}

	// Add to allowed peers
	err = c.db.AddPeer(*peerInfo)
	if err != nil {
		return fmt.Errorf("error adding peer to database: %v", err)
	}

	// Also add to p2pNode's in-memory map
	c.p2pNode.AddPeer(*peerInfo)

	fmt.Println("Successfully bound peer:", peerInfo.ID)

	// Publish peer update to network
	addrs := []string{}
	for _, addr := range peerInfo.Addrs {
		addrs = append(addrs, addr.String())
	}
	if err := c.PublishPeerUpdate("ADD", peerInfo.ID.String(), addrs); err != nil {
		fmt.Println("Warning: Failed to publish peer update:", err)
	}

	return nil
}

// bindToMaster is called by replica nodes to receive authorization from a master node
func (c *Core) bindToMaster() {
	clientInfo, err := p2p.BindToClient(c.p2pNode)
	if err != nil {
		panic(fmt.Sprintf("Error binding to master: %v", err))
	}

	// Store master public key
	err = c.db.SetMasterPublicKey(clientInfo.MasterPublicKey)
	if err != nil {
		panic(fmt.Sprintf("Error storing master public key: %v", err))
	}

	// Update keys with received master public key
	c.keys.MasterPublicKey = clientInfo.MasterPublicKey

	// Store the updated keys
	c.db.StoreKeys(c.keys)

	// Add master node to allowed peers
	err = c.db.AddPeer(clientInfo.AddrInfo)
	if err != nil {
		panic(fmt.Sprintf("Error adding master peer: %v", err))
	}

	// Store all peers from the received list
	for _, peerInfo := range clientInfo.PeerList {
		if err := c.db.AddPeer(peerInfo); err != nil {
			fmt.Printf("Warning: Failed to add peer %s: %v\n", peerInfo.ID, err)
		}
	}

	// Update P2P node's in-memory peer map with all peers (including master)
	allPeers := append(clientInfo.PeerList, clientInfo.AddrInfo)
	c.p2pNode.ReplacePeers(allPeers)

	fmt.Println("Successfully bound to master node:", clientInfo.PeerID)
	fmt.Printf("Received %d peers from network\n", len(clientInfo.PeerList))
	fmt.Println("Note: This replica node does not have the encryption key and cannot decrypt data")
}

// coreStartupWithMnemonic initializes a core with a specific mnemonic
func coreStartupWithMnemonic(mnemonic string) *Core {
	c := &Core{
		db: database.Create(),
	}

	keys := c.db.GetKeys()
	if keys == nil {
		keys = crypto.SetupKeysFromMnemonic(mnemonic)
		c.db.StoreKeys(keys)
		fmt.Println("Initialized keys from mnemonic")
	}

	ctx := context.Background()
	p2pNode, err := p2p.NewP2PNode(keys.PeerPrivateKey, ctx, c.db.GetPeers(), 13000)
	if err != nil {
		panic(fmt.Sprintf("Error starting P2P node: %v", err))
	}

	c.p2pNode = p2pNode
	c.keys = keys
	c.storage = storage.NewStorage(c.db, keys.AESKey)

	return c
}

// RequestLatestUpdate sends a request to all peers for their latest update
func (c *Core) RequestLatestUpdate() {
	c.notify("request_latest_update", nil)
}

// PublishDataUpdate creates and broadcasts a data update (ADD or DELETE)
func (c *Core) PublishDataUpdate(action string, key, value []byte, size int64, hash []byte) error {
	if c.keys.MasterPrivateKey == nil {
		return fmt.Errorf("only master nodes can publish data updates")
	}

	// Get current state
	currentID, err := c.db.GetCurrentUpdateID()
	if err != nil {
		currentID = 0
	}

	prevDataHash, err := c.db.GetDataRootHash()
	if err != nil {
		prevDataHash = make([]byte, 32)
	}

	prevPeerHash, err := c.db.GetPeerListHash()
	if err != nil {
		prevPeerHash = make([]byte, 32)
	}

	// Create DataUpdate
	dataUpdate := DataUpdate{
		Action: action,
		Key:    key,
		Value:  value,
		Size:   size,
		Hash:   hash,
	}

	// Update merkle tree (data is already in DB from the storage layer)
	switch action {
	case "ADD", "MODIFY":
		c.merkleTree.Insert(hash)
	case "DELETE":
		c.merkleTree.Delete(hash)
	}
	c.updateDataHash()

	// Get new data hash from merkle tree
	newDataHash := c.merkleTree.GetRootHash()

	// Create update
	update := Update{
		UpdateID:         currentID + 1,
		PeerListHash:     prevPeerHash,
		PrevPeerListHash: prevPeerHash,
		DataHash:         newDataHash,
		PrevDataHash:     prevDataHash,
		NumBuckets:       c.merkleTree.GetNumBuckets(),
		UpdateDataType:   "DATA",
		UpdateData:       dataUpdate,
		Timestamp:        time.Now().Unix(),
	}

	// Sign update
	signedUpdate, err := SignUpdate(update, c.keys.MasterPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign update: %w", err)
	}

	// Store update
	signedUpdateJSON, err := json.Marshal(signedUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal signed update: %w", err)
	}
	if err := c.db.InsertSignedUpdate(update.UpdateID, string(signedUpdateJSON)); err != nil {
		return fmt.Errorf("failed to insert update: %w", err)
	}

	// Update node state
	c.db.SetCurrentUpdateID(update.UpdateID)
	c.db.SetDataRootHash(newDataHash)
	c.db.SetLatestUpdateJSON(string(signedUpdateJSON))

	// Broadcast notification
	return c.notify("update", signedUpdateJSON)
}

// PublishPeerUpdate creates and broadcasts a peer update (ADD or REMOVE)
func (c *Core) PublishPeerUpdate(action string, peerID string, addrs []string) error {
	// Get current state
	currentID, err := c.db.GetCurrentUpdateID()
	if err != nil {
		currentID = 0
	}

	prevPeerHash, err := c.db.GetPeerListHash()
	if err != nil {
		prevPeerHash = make([]byte, 32)
	}

	prevDataHash, err := c.db.GetDataRootHash()
	if err != nil {
		prevDataHash = make([]byte, 32)
	}

	// Compute new peer list hash
	newPeerHash := ComputePeerListHash(c.db.GetAllPeerIDs())

	// Create update data
	peerUpdate := PeerUpdate{
		Action:    action,
		PeerID:    peerID,
		Addresses: addrs,
	}

	// Create update
	update := Update{
		UpdateID:         currentID + 1,
		PeerListHash:     newPeerHash,
		PrevPeerListHash: prevPeerHash,
		DataHash:         prevDataHash,
		PrevDataHash:     prevDataHash,
		UpdateDataType:   "PEER",
		UpdateData:       peerUpdate,
		Timestamp:        time.Now().Unix(),
	}

	// Sign entire update JSON
	signedUpdate, err := SignUpdate(update, c.keys.MasterPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign update: %w", err)
	}

	// Store update
	signedUpdateJSON, err := json.Marshal(signedUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal signed update: %w", err)
	}
	if err := c.db.InsertSignedUpdate(update.UpdateID, string(signedUpdateJSON)); err != nil {
		return fmt.Errorf("failed to insert update: %w", err)
	}

	// Update node state
	c.db.SetCurrentUpdateID(update.UpdateID)
	c.db.SetPeerListHash(newPeerHash)
	c.db.SetLatestUpdateJSON(string(signedUpdateJSON))

	// Broadcast notification
	notificationJSON, err := json.Marshal(signedUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal signed update: %w", err)
	}

	return c.notify("update", notificationJSON)
}
