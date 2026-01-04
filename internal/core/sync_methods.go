package core

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/notassigned/endershare/internal/crypto"
)

const FILE_STREAM_CHUNK_SIZE = 64 * 1024

// HandleGetPeerList returns the full peer list (for p2p handler)
func (c *Core) HandleGetPeerList() []PeerInfoResponse {
	peers := c.db.GetPeers()

	response := []PeerInfoResponse{}
	for _, peerInfo := range peers {
		addrs := []string{}
		for _, addr := range peerInfo.Addrs {
			addrs = append(addrs, addr.String())
		}

		response = append(response, PeerInfoResponse{
			PeerID:    peerInfo.ID.String(),
			Addresses: addrs,
		})
	}

	return response
}

// HandleGetTreeBucketHashes returns merkle tree bucket hashes
func (c *Core) HandleGetTreeBucketHashes(numBuckets int) [][]byte {
	if c.merkleTree == nil || c.merkleTree.GetNumBuckets() != numBuckets {
		// Tree structure mismatch
		return [][]byte{}
	}

	return c.merkleTree.GetBucketHashes()
}

// HandleGetDataBucketHashes returns data entry hashes for a bucket
func (c *Core) HandleGetDataBucketHashes(bucketIdx int, numBuckets int) [][]byte {
	return c.db.GetBucketHashes(bucketIdx, numBuckets)
}

// Stream handler methods for p2p protocol handlers

// handlePeerListRequest handles requests for the full peer list
func (c *Core) handlePeerListRequest(s network.Stream) {
	defer s.Close()

	response := c.HandleGetPeerList()

	encoder := json.NewEncoder(s)
	encoder.Encode(response)
}

// handleTreeBucketHashesRequest handles requests for merkle tree bucket hashes
func (c *Core) handleTreeBucketHashesRequest(s network.Stream) {
	defer s.Close()

	var req TreeBucketHashesRequest
	decoder := json.NewDecoder(s)
	if err := decoder.Decode(&req); err != nil {
		return
	}

	response := c.HandleGetTreeBucketHashes(req.NumBuckets)

	encoder := json.NewEncoder(s)
	encoder.Encode(response)
}

// handleDataBucketHashesRequest handles requests for data bucket hashes
func (c *Core) handleDataBucketHashesRequest(s network.Stream) {
	defer s.Close()

	var req DataBucketHashesRequest
	decoder := json.NewDecoder(s)
	if err := decoder.Decode(&req); err != nil {
		return
	}

	response := c.HandleGetDataBucketHashes(req.BucketIndex, req.NumBuckets)

	encoder := json.NewEncoder(s)
	encoder.Encode(response)
}

// handleMetadataRequest handles requests for metadata (key+value) by hash list
func (c *Core) handleMetadataRequest(s network.Stream) {
	defer s.Close()

	encoder := json.NewEncoder(s)
	buf := make([]byte, 8192)

	for {
		// Read batch of hashes into buffer (8192 bytes = 256 hashes max per batch)
		n, err := io.ReadFull(s, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			// Client finished sending hashes
			if n == 0 {
				break
			}
			// Process partial buffer if we got some data
		} else if err != nil {
			// Other error
			return
		}

		// Parse hashes from buffer (each hash is 32 bytes)
		if n%32 != 0 {
			return //invalid data
		}

		numHashes := n / 32
		hashes := make([][]byte, numHashes)
		for i := 0; i < numHashes; i++ {
			hash := make([]byte, 32)
			copy(hash, buf[i*32:(i+1)*32])
			hashes[i] = hash
		}

		// Query database in batch
		entries := c.db.GetDataByHashes(hashes)

		// Check if we got all requested hashes
		if len(entries) != len(hashes) {
			// Some hash not found - close stream immediately
			return
		}

		// Send each entry as individual JSON object
		for _, entry := range entries {
			metaEntry := MetadataEntry{
				Hash:  entry.Hash,
				Key:   entry.Key,
				Value: entry.Value,
				Size:  entry.Size,
			}

			if err := encoder.Encode(metaEntry); err != nil {
				return
			}
		}

		// If we got less than a full buffer, we're done
		if n < 8192 {
			break
		}
	}
}

// handleFileDataRequest handles requests for file data with offset support
func (c *Core) handleFileDataRequest(s network.Stream) {
	defer s.Close()

	// Decode request
	var req FileDataRequest
	decoder := json.NewDecoder(s)
	if err := decoder.Decode(&req); err != nil {
		return
	}

	if c.storage == nil {
		return
	}

	// Open file for reading
	file, totalSize, err := c.storage.OpenFileForReading(req.FileHash)
	if err != nil {
		return
	}
	defer file.Close()

	// Seek to requested offset
	if _, err := file.Seek(req.Offset, 0); err != nil {
		return
	}

	// Determine how much to read
	remaining := totalSize - req.Offset
	if req.Length > 0 && req.Length < remaining {
		remaining = req.Length
	}

	// Stream file in 64KB chunks
	buf := make([]byte, FILE_STREAM_CHUNK_SIZE)

	for remaining > 0 {
		toRead := FILE_STREAM_CHUNK_SIZE
		if int64(toRead) > remaining {
			toRead = int(remaining)
		}

		n, err := file.Read(buf[:toRead])
		if err != nil && err != io.EOF {
			return
		}
		if n == 0 {
			break
		}

		if _, err := s.Write(buf[:n]); err != nil {
			return
		}

		remaining -= int64(n)
	}
}

// MetadataEntry represents a data table entry for protocol response
type MetadataEntry struct {
	Hash  []byte `json:"hash"`
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
	Size  int64  `json:"size"`
}

// PeerInfoResponse represents peer information for protocol response
type PeerInfoResponse struct {
	PeerID    string   `json:"peer_id"`
	Addresses []string `json:"addresses"`
}

// TreeBucketHashesRequest requests merkle tree bucket hashes
type TreeBucketHashesRequest struct {
	NumBuckets int `json:"num_buckets"`
}

// DataBucketHashesRequest requests data entry hashes for a bucket
type DataBucketHashesRequest struct {
	BucketIndex int `json:"bucket_index"`
	NumBuckets  int `json:"num_buckets"`
}

// FileDataRequest requests file data with offset support
type FileDataRequest struct {
	FileHash []byte `json:"file_hash"`
	Offset   int64  `json:"offset"`
	Length   int64  `json:"length"`
}

// Request/response helper methods for making sync requests to peers

// RequestTreeBucketHashes requests merkle tree bucket hashes from a peer
func (c *Core) RequestTreeBucketHashes(from peer.ID, numBuckets int) [][]byte {
	// TODO: Implement libp2p stream request
	return [][]byte{}
}

// RequestDataBucketHashes requests data entry hashes for a bucket from a peer
func (c *Core) RequestDataBucketHashes(from peer.ID, bucketIdx int, numBuckets int) [][]byte {
	// TODO: Implement libp2p stream request
	return [][]byte{}
}

// RequestMetadata requests metadata for a list of hashes from a peer
// Returns partial results if peer closes stream early (missing hash)
func (c *Core) RequestMetadata(from peer.ID, hashes [][]byte) ([]MetadataEntry, error) {
	if len(hashes) == 0 {
		return []MetadataEntry{}, nil
	}

	// Open stream to peer
	stream, err := c.p2pNode.NewStreamToPeer(from, "/endershare/metadata/1.0")
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	// Write all hashes as 32-byte buffers
	for _, hash := range hashes {
		if len(hash) != 32 {
			return nil, fmt.Errorf("invalid hash length: %d", len(hash))
		}
		if _, err := stream.Write(hash); err != nil {
			return nil, err
		}
	}

	stream.CloseWrite()

	// Read responses - collect all available entries
	decoder := json.NewDecoder(stream)
	entries := make([]MetadataEntry, 0, len(hashes))

	for i := 0; i < len(hashes); i++ {
		var entry MetadataEntry
		if err := decoder.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Verify hash matches
		computedHash := crypto.ComputeDataHash(entry.Key, entry.Value, entry.Size)

		if !bytes.Equal(computedHash, entry.Hash) {
			return nil, fmt.Errorf("hash verification failed for entry")
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// RequestFileData requests file data from a peer with offset support
func (c *Core) RequestFileData(from peer.ID, fileHash []byte, offset int64, length int64) ([]byte, int64, error) {
	// TODO: Implement libp2p stream request
	return []byte{}, 0, nil
}

// containsHash checks if a hash exists in a slice of hashes
func containsHash(hashes [][]byte, target []byte) bool {
	for _, h := range hashes {
		if bytes.Equal(h, target) {
			return true
		}
	}
	return false
}

// Data mutation methods that maintain both database and merkle tree

// insertData inserts a data entry and updates the merkle tree
func (c *Core) insertData(key, value []byte, size int64, hash []byte) error {
	c.db.PutData(key, value, size, hash)
	c.merkleTree.Insert(hash)
	return nil
}

// deleteData deletes a data entry and updates the merkle tree
func (c *Core) deleteData(key, hash []byte) error {
	c.db.DeleteData(key)
	c.merkleTree.Delete(hash)
	return nil
}

// updateDataHash updates the data_hash node property from the merkle tree root
func (c *Core) updateDataHash() {
	rootHash := c.merkleTree.GetRootHash()
	c.db.SetNodeProperty("data_hash", base64Encode(rootHash))
}

// downloadFile downloads a file from a peer with resumable support
func (c *Core) downloadFile(from peer.ID, fileHash []byte) error {
	if c.storage == nil {
		return nil
	}

	// Check if file already exists
	if c.storage.FileExists(fileHash) {
		return nil
	}

	// Check for partial download
	offset := c.db.GetDownloadProgress(fileHash)
	if offset < 0 {
		// Already complete
		return nil
	}

	for {
		// Request chunk (1MB at a time)
		data, totalSize, err := c.RequestFileData(from, fileHash, offset, 1024*1024)
		if err != nil {
			return err
		}

		// Append to file
		if err := c.storage.AppendFileData(fileHash, data); err != nil {
			return err
		}

		offset += int64(len(data))

		// Update progress
		c.db.SetDownloadProgress(fileHash, offset)

		// Check if complete
		if offset >= totalSize {
			c.db.SetDownloadProgress(fileHash, -1) // Mark complete
			break
		}

		// If no more data returned, we're done
		if len(data) == 0 {
			break
		}
	}

	return nil
}

// Helper functions

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
