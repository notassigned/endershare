package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/notassigned/endershare/internal/database"
)

// processUpdate is called when an update is received via gossipsub
func (c *Core) processUpdate(signedUpdate SignedUpdate, from peer.ID) error {
	// 1. Verify signature
	if !VerifySignedUpdate(signedUpdate, c.keys.MasterPublicKey) {
		return fmt.Errorf("invalid update signature")
	}

	// 2. Check if we've already processed this update
	currentIDStr, err := c.db.GetNodeProperty("current_update_id")
	if err != nil {
		currentIDStr = "0"
	}
	currentID, _ := strconv.ParseUint(currentIDStr, 10, 64)

	if signedUpdate.Update.UpdateID <= currentID {
		return nil
	}

	// 3. Sync peer list if needed
	if err := c.syncPeerList(signedUpdate.Update, from); err != nil {
		return fmt.Errorf("failed to sync peer list: %w", err)
	}

	// 4. Sync data if needed
	if err := c.syncData(signedUpdate.Update, from); err != nil {
		return fmt.Errorf("failed to sync data: %w", err)
	}

	// 5. Update node state
	c.db.SetNodeProperty("current_update_id", fmt.Sprintf("%d", signedUpdate.Update.UpdateID))
	c.db.SetNodeProperty("peer_list_hash", base64.StdEncoding.EncodeToString(signedUpdate.Update.PeerListHash))
	c.db.SetNodeProperty("data_hash", base64.StdEncoding.EncodeToString(signedUpdate.Update.DataHash))

	// 6. Store update in database
	signedUpdateJSON, err := json.Marshal(signedUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal signed update: %w", err)
	}
	c.db.InsertSignedUpdate(signedUpdate.Update.UpdateID, string(signedUpdateJSON))

	return nil
}

// syncPeerList handles peer list synchronization
func (c *Core) syncPeerList(update Update, from peer.ID) error {
	// Get current peer list hash
	currentHashStr, err := c.db.GetNodeProperty("peer_list_hash")
	if err != nil {
		currentHashStr = base64.StdEncoding.EncodeToString(make([]byte, 32))
	}
	currentHash, _ := base64.StdEncoding.DecodeString(currentHashStr)

	// Check if peer list hash differs
	if bytes.Equal(update.PeerListHash, currentHash) {
		// Peer list is already in sync
		return nil
	}

	// Check if we can fast-forward
	if bytes.Equal(update.PrevPeerListHash, currentHash) && update.UpdateDataType == "PEER" {
		// Fast-forward: apply update directly
		return c.applyPeerUpdate(update.UpdateData, update.PeerListHash, from)
	}

	// Full sync needed: request entire peer list
	return c.syncPeerListFull(update.PeerListHash, from)
}

// applyPeerUpdate applies a peer update directly (fast-forward path)
func (c *Core) applyPeerUpdate(updateData interface{}, expectedHash []byte, from peer.ID) error {
	// Parse as PeerUpdate
	updateJSON, err := json.Marshal(updateData)
	if err != nil {
		return err
	}

	var peerUpdate PeerUpdate
	if err := json.Unmarshal(updateJSON, &peerUpdate); err != nil {
		return err
	}

	switch peerUpdate.Action {
	case "ADD":
		// Check if peer already exists
		existingPeers := c.db.GetAllPeerIDs()
		peerExists := false
		for _, id := range existingPeers {
			if id == peerUpdate.PeerID {
				peerExists = true
				break
			}
		}

		if peerExists {
			// Update addresses
			c.db.UpdatePeerAddresses(peerUpdate.PeerID, peerUpdate.Addresses)
		} else {
			// Add new peer
			// Convert to AddrInfo format (simplified - just store in database directly)
			// The full conversion will happen when GetPeers is called
			// For now, use the raw insert
			c.db.AddPeer(peerInfoFromPeerUpdate(peerUpdate), peerUpdate.PeerSignature)
		}

	case "REMOVE":
		c.db.RemovePeer(peerUpdate.PeerID)

	default:
		return fmt.Errorf("unknown peer update action: %s", peerUpdate.Action)
	}

	// Verify the new peer list hash matches, if not pull full list
	currentHash := ComputePeerListHash(c.db.GetAllPeerIDs())
	if !bytes.Equal(currentHash, expectedHash) {
		return c.syncPeerListFull(expectedHash, from)
	}

	return nil
}

// syncPeerListFull requests the full peer list from a peer
func (c *Core) syncPeerListFull(expectedHash []byte, from peer.ID) error {
	resp, err := c.RequestPeerList(from)
	if err != nil {
		return err
	}

	// Convert response to DBPeer slice
	dbPeers := make([]database.DBPeer, len(resp))
	for i, p := range resp {
		dbPeers[i] = database.DBPeer{
			PeerID:        p.PeerID,
			Addresses:     p.Addresses,
			PeerSignature: p.PeerSignature,
		}
	}

	// Atomically replace all peers
	if err := c.db.ReplaceAllPeers(dbPeers); err != nil {
		return fmt.Errorf("failed to replace peers: %w", err)
	}

	// Verify the new peer list hash matches
	currentHash := ComputePeerListHash(c.db.GetAllPeerIDs())
	if !bytes.Equal(currentHash, expectedHash) {
		return fmt.Errorf("peer list hash mismatch after sync")
	}

	return nil
}

// syncData handles data synchronization
func (c *Core) syncData(update Update, from peer.ID) error {
	// Get current data hash
	currentHashStr, err := c.db.GetNodeProperty("data_hash")
	if err != nil {
		currentHashStr = base64.StdEncoding.EncodeToString(make([]byte, 32))
	}
	currentHash, _ := base64.StdEncoding.DecodeString(currentHashStr)

	// Check if data hash differs
	if bytes.Equal(update.DataHash, currentHash) {
		// Data is already in sync
		return nil
	}

	// Check if we can fast-forward
	if bytes.Equal(update.PrevDataHash, currentHash) && update.UpdateDataType == "DATA" {
		// Fast-forward: apply update directly
		return c.applyDataUpdate(update.UpdateData)
	}

	// Full sync needed: use merkle tree diff (future implementation)
	return c.syncDataFull(update.DataHash, from)
}

// applyDataUpdate applies a data update directly (fast-forward path)
func (c *Core) applyDataUpdate(updateData interface{}) error {
	// Parse as DataUpdate
	updateJSON, err := json.Marshal(updateData)
	if err != nil {
		return err
	}

	var dataUpdate DataUpdate
	if err := json.Unmarshal(updateJSON, &dataUpdate); err != nil {
		// Not a data update, might be a peer update - ignore
		return nil
	}

	switch dataUpdate.Action {
	case "ADD", "MODIFY":
		// Add/update entry in data table
		// TODO: Request file by hash if needed
		fmt.Printf("Data update: %s key with hash\n", dataUpdate.Action)

	case "DELETE":
		// Remove entry from data table
		fmt.Printf("Data update: DELETE key\n")

	default:
		return fmt.Errorf("unknown data update action: %s", dataUpdate.Action)
	}

	return nil
}

// syncDataFull performs full data sync using merkle tree
func (c *Core) syncDataFull(expectedHash []byte, from peer.ID) error {
	// TODO: Implement merkle tree sync
	fmt.Println("Warning: Full data sync not yet implemented")
	return nil
}

// Helper to convert PeerUpdate to peer.AddrInfo
func peerInfoFromPeerUpdate(pu PeerUpdate) peer.AddrInfo {
	var addrs []multiaddr.Multiaddr
	for _, addrStr := range pu.Addresses {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err == nil {
			addrs = append(addrs, addr)
		}
	}
	return peer.AddrInfo{ID: peer.ID(pu.PeerID), Addrs: addrs}
}

// RequestPeerList requests the full peer list from a connected peer
func (c *Core) RequestPeerList(peerID peer.ID) ([]PeerInfoResponse, error) {
	// Open stream to peer
	stream, err := c.p2pNode.GetHost().NewStream(
		context.Background(),
		peer.ID(peerID),
		protocol.ID("/endershare/peer-list/1.0"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Read response
	var response []PeerInfoResponse
	decoder := json.NewDecoder(stream)
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}

type PeerInfoResponse struct {
	PeerID        string   `json:"peer_id"`
	Addresses     []string `json:"addresses"`
	PeerSignature []byte   `json:"peer_signature"`
}
