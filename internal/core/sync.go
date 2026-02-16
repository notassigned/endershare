package core

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
)

// processUpdate is called when an update is received via gossipsub
func (c *Core) processUpdate(signedUpdate SignedUpdate, from peer.ID) error {
	// 1. Verify signature
	if !VerifySignedUpdate(signedUpdate, c.keys.MasterPublicKey) {
		return fmt.Errorf("invalid update signature")
	}

	// 2. Parse the update
	update, err := signedUpdate.GetUpdate()
	if err != nil {
		return fmt.Errorf("failed to parse update: %w", err)
	}

	// 3. Check if we've already processed this update
	currentID, err := c.db.GetCurrentUpdateID()
	if err != nil {
		currentID = 0
	}

	if update.UpdateID <= currentID {
		return nil
	}

	// 4. Sync peer list if needed
	if err := c.syncPeerList(update, from); err != nil {
		return fmt.Errorf("failed to sync peer list: %w", err)
	}

	// 5. Sync data if needed
	if err := c.syncData(update, from); err != nil {
		return fmt.Errorf("failed to sync data: %w", err)
	}

	// 6. Update node state
	c.db.SetCurrentUpdateID(update.UpdateID)
	c.db.SetPeerListHash(update.PeerListHash)
	c.db.SetDataRootHash(update.DataHash)

	// 7. Store update in database
	signedUpdateJSON, err := json.Marshal(signedUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal signed update: %w", err)
	}
	c.db.SetLatestUpdateJSON(string(signedUpdateJSON))

	//Log to db for debug
	c.db.InsertSignedUpdate(update.UpdateID, string(signedUpdateJSON))

	// Reload folder ID counter so new folders don't collide with synced ones
	if c.storage != nil {
		c.storage.ReloadNextFolderID()
	}

	// Notify UI of data change
	if c.OnDataUpdated != nil {
		c.OnDataUpdated()
	}

	return nil
}

// syncPeerList handles peer list synchronization
func (c *Core) syncPeerList(update Update, from peer.ID) error {
	// Get current peer list hash
	currentHash, err := c.db.GetPeerListHash()
	if err != nil {
		currentHash = make([]byte, 32)
	}

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
			c.db.AddPeer(peerInfoFromPeerUpdate(peerUpdate))
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
			PeerID:    p.PeerID,
			Addresses: p.Addresses,
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
	currentHash, err := c.db.GetDataRootHash()
	if err != nil {
		currentHash = make([]byte, 32)
	}

	// Check if data hash differs
	if bytes.Equal(update.DataHash, currentHash) {
		// Data is already in sync
		return nil
	}

	// Check if we can fast-forward
	if bytes.Equal(update.PrevDataHash, currentHash) && update.UpdateDataType == "DATA" {
		// Fast-forward: apply update directly
		return c.applyDataUpdate(update.UpdateData, from)
	}

	// Full sync needed: use merkle tree diff
	return c.syncDataFull(update, from)
}

// applyDataUpdate applies a data update directly (fast-forward path)
func (c *Core) applyDataUpdate(updateData interface{}, from peer.ID) error {
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
		// Insert metadata into database and merkle tree
		c.insertData(dataUpdate.Key, dataUpdate.Value, dataUpdate.Size, dataUpdate.Hash)

		// Download file if Value is not nil (folders have nil value)
		if dataUpdate.Value != nil {
			if err := c.downloadFile(from, dataUpdate.Value, dataUpdate.Size); err != nil {
				fmt.Printf("Warning: failed to download file: %v\n", err)
			}
		}

	case "DELETE":
		// Remove entry from database and merkle tree
		c.deleteData(dataUpdate.Key, dataUpdate.Hash)

	default:
		return fmt.Errorf("unknown data update action: %s", dataUpdate.Action)
	}

	// Update data hash once after applying update
	c.updateDataHash()

	return nil
}

// syncDataFull performs full data sync using merkle tree
func (c *Core) syncDataFull(update Update, from peer.ID) error {
	// Phase 1: Check if tree structure matches
	if c.merkleTree == nil || c.merkleTree.GetNumBuckets() != update.NumBuckets {
		// Bucket count mismatch - need full rebuild
		return c.rebuildTreeFromPeer(update.NumBuckets, update.DataHash, from)
	}

	// Phase 2: Request peer's merkle tree bucket hashes and find differences
	peerTreeBuckets := c.RequestTreeBucketHashes(from, update.NumBuckets)
	localTreeBuckets := c.merkleTree.GetBucketHashes()

	diffBucketIndices := []int{}
	for i := 0; i < len(localTreeBuckets); i++ {
		if i >= len(peerTreeBuckets) || !bytes.Equal(localTreeBuckets[i], peerTreeBuckets[i]) {
			diffBucketIndices = append(diffBucketIndices, i)
		}
	}

	// Phase 3: For each differing bucket, get data entry hashes and compute diff
	c.db.MarkAllStale() // Mark all entries as stale

	hashesToDownload := [][]byte{} // Data entry hashes needing metadata/files

	// Batch request all differing buckets at once
	peerBucketHashes, err := c.RequestDataBucketHashes(from, diffBucketIndices, update.NumBuckets)
	if err != nil {
		return err
	}

	for _, bucketIdx := range diffBucketIndices {
		// Get peer and local data hashes for this bucket
		peerDataHashes := peerBucketHashes[bucketIdx]
		localDataHashes := c.db.GetBucketHashes(bucketIdx, update.NumBuckets)

		// Mark peer hashes as current
		for _, hash := range peerDataHashes {
			if !containsHash(localDataHashes, hash) {
				// New hash - need to download metadata and file
				hashesToDownload = append(hashesToDownload, hash)
			}
			// Mark as current (will be inserted or already exists)
			c.db.MarkHashCurrent(hash)
		}
	}

	// Phase 4: Download metadata and files for new hashes
	if len(hashesToDownload) > 0 {
		// Request metadata (key + file hash) for all needed hashes at once
		metadataList, err := c.RequestMetadata(from, hashesToDownload)
		if err != nil {
			return fmt.Errorf("failed to request metadata: %w", err)
		}

		for _, metadata := range metadataList {
			// Insert metadata into database
			c.db.PutData(metadata.Key, metadata.Value, metadata.Size, metadata.Hash)
			c.merkleTree.Insert(metadata.Hash)

			// Request file if Value is not nil (folders have nil value)
			if metadata.Value != nil {
				if err := c.downloadFile(from, metadata.Value, metadata.Size); err != nil {
					fmt.Printf("Warning: failed to download file: %v\n", err)
				}
			}
		}
	}

	// Phase 5: Delete stale entries and update hash
	staleHashes := c.db.GetStaleHashes()
	for _, hash := range staleHashes {
		c.merkleTree.Delete(hash)
	}
	c.db.DeleteStaleEntries()

	c.updateDataHash() // Call once at end

	// Verify root hash
	if !bytes.Equal(c.merkleTree.GetRootHash(), update.DataHash) {
		return fmt.Errorf("merkle root mismatch after sync")
	}

	return nil
}

// rebuildTreeFromPeer performs a full rebuild when bucket count mismatches.
// Requests all data from the peer and rebuilds the local merkle tree with the peer's bucket count.
func (c *Core) rebuildTreeFromPeer(numBuckets int, expectedHash []byte, from peer.ID) error {
	// Build list of all bucket indices
	allIndices := make([]int, numBuckets)
	for i := range allIndices {
		allIndices[i] = i
	}

	// Request all data entry hashes from peer
	peerBucketHashes, err := c.RequestDataBucketHashes(from, allIndices, numBuckets)
	if err != nil {
		return err
	}

	// Collect all peer hashes and determine which are new
	c.db.MarkAllStale()
	var hashesToDownload [][]byte
	var allPeerHashes [][]byte

	for _, bucketIdx := range allIndices {
		localHashes := c.db.GetBucketHashes(bucketIdx, numBuckets)
		for _, hash := range peerBucketHashes[bucketIdx] {
			allPeerHashes = append(allPeerHashes, hash)
			c.db.MarkHashCurrent(hash)
			if !containsHash(localHashes, hash) {
				hashesToDownload = append(hashesToDownload, hash)
			}
		}
	}

	// Download metadata and files for new hashes
	if len(hashesToDownload) > 0 {
		metadataList, err := c.RequestMetadata(from, hashesToDownload)
		if err != nil {
			return fmt.Errorf("failed to request metadata: %w", err)
		}
		for _, metadata := range metadataList {
			c.db.PutData(metadata.Key, metadata.Value, metadata.Size, metadata.Hash)
			if metadata.Value != nil {
				if err := c.downloadFile(from, metadata.Value, metadata.Size); err != nil {
					fmt.Printf("Warning: failed to download file: %v\n", err)
				}
			}
		}
	}

	// Delete stale entries
	c.db.DeleteStaleEntries()

	// Rebuild merkle tree with peer's bucket count
	c.merkleTree = crypto.NewMerkleTreeWithBuckets(allPeerHashes, numBuckets)
	c.updateDataHash()

	if !bytes.Equal(c.merkleTree.GetRootHash(), expectedHash) {
		return fmt.Errorf("merkle root mismatch after rebuild")
	}

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
	stream, err := c.p2pNode.NewStreamToPeer(
		peer.ID(peerID),
		"/endershare/peer-list/1.0",
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
