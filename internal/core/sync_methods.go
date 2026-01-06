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

// Stream handler methods for p2p protocol handlers

// handlePeerListRequest handles requests for the full peer list
func (c *Core) handlePeerListRequest(s network.Stream) {
	defer s.Close()

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
	var response [][]byte
	if c.merkleTree == nil || c.merkleTree.GetNumBuckets() != req.NumBuckets {
		// Tree structure mismatch
		return
	} else {
		response = c.merkleTree.GetBucketHashes()
	}

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

	// Build response for each requested bucket
	response := make([]DataBucketHashesResponse, 0, len(req.BucketIndices))
	for _, bucketIdx := range req.BucketIndices {
		hashes := c.db.GetBucketHashes(bucketIdx, req.NumBuckets)
		response = append(response, DataBucketHashesResponse{
			BucketIndex: bucketIdx,
			Hashes:      hashes,
		})
	}

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

// DataBucketHashesRequest requests data entry hashes for multiple buckets
type DataBucketHashesRequest struct {
	BucketIndices []int `json:"bucket_indices"`
	NumBuckets    int   `json:"num_buckets"`
}

// DataBucketHashesResponse contains hashes for a specific bucket
type DataBucketHashesResponse struct {
	BucketIndex int      `json:"bucket_index"`
	Hashes      [][]byte `json:"hashes"`
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
	// Open stream to peer
	stream, err := c.p2pNode.NewStreamToPeer(from, "/endershare/tree-bucket-hashes/1.0")
	if err != nil {
		return [][]byte{}
	}
	defer stream.Close()

	// Encode request
	req := TreeBucketHashesRequest{NumBuckets: numBuckets}
	encoder := json.NewEncoder(stream)
	if err := encoder.Encode(req); err != nil {
		return [][]byte{}
	}

	// Decode response
	var response [][]byte
	decoder := json.NewDecoder(stream)
	if err := decoder.Decode(&response); err != nil {
		return [][]byte{}
	}

	return response
}

// RequestDataBucketHashes requests data entry hashes for multiple buckets from a peer
func (c *Core) RequestDataBucketHashes(from peer.ID, bucketIndices []int, numBuckets int) map[int][][]byte {
	if len(bucketIndices) == 0 {
		return map[int][][]byte{}
	}

	// Open stream to peer
	stream, err := c.p2pNode.NewStreamToPeer(from, "/endershare/data-bucket-hashes/1.0")
	if err != nil {
		return map[int][][]byte{}
	}
	defer stream.Close()

	// Encode request
	req := DataBucketHashesRequest{
		BucketIndices: bucketIndices,
		NumBuckets:    numBuckets,
	}
	encoder := json.NewEncoder(stream)
	if err := encoder.Encode(req); err != nil {
		return map[int][][]byte{}
	}

	// Decode response
	var response []DataBucketHashesResponse
	decoder := json.NewDecoder(stream)
	if err := decoder.Decode(&response); err != nil {
		return map[int][][]byte{}
	}

	// Convert response array to map
	result := make(map[int][][]byte, len(response))
	for _, bucketResp := range response {
		result[bucketResp.BucketIndex] = bucketResp.Hashes
	}

	return result
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
	c.db.SetNodeProperty("data_hash", base64.StdEncoding.EncodeToString(rootHash))
}

// downloadFile downloads a file from a peer with resumable support
func (c *Core) downloadFile(from peer.ID, fileHash []byte, fileSize int64) error {
	if c.storage == nil {
		return nil
	}

	offset := c.db.GetDownloadProgress(fileHash)
	if offset == fileSize {
		return nil
	}

	stream, err := c.p2pNode.NewStreamToPeer(from, "/endershare/file-data/1.0")
	if err != nil {
		return err
	}
	defer stream.Close()

	req := FileDataRequest{
		FileHash: fileHash,
		Offset:   offset,
		Length:   fileSize - offset,
	}

	encoder := json.NewEncoder(stream)
	if err := encoder.Encode(req); err != nil {
		return err
	}

	const WRITE_BUFFER_SIZE = 20 * 1024 * 1024
	buffer := make([]byte, 0, WRITE_BUFFER_SIZE)
	chunk := make([]byte, FILE_STREAM_CHUNK_SIZE)
	totalWritten := int64(0)
	eof := false

	for totalWritten < req.Length {
		buffer = buffer[:0] // Reuse buffer capacity

		for len(buffer) < WRITE_BUFFER_SIZE && totalWritten+int64(len(buffer)) < req.Length && !eof {
			n, err := stream.Read(chunk)
			if n > 0 {
				buffer = append(buffer, chunk[:n]...)
			}
			if err != nil {
				if err == io.EOF {
					eof = true
					break
				}
				return err
			}
		}

		if len(buffer) == 0 {
			break
		}

		if err := c.storage.AppendFileData(fileHash, buffer); err != nil {
			return err
		}

		totalWritten += int64(len(buffer))

		if err := c.db.SetDownloadProgress(fileHash, offset+totalWritten); err != nil {
			return err
		}
	}

	if totalWritten != req.Length {
		return fmt.Errorf("incomplete download: expected %d bytes, got %d", req.Length, totalWritten)
	}

	if err := c.db.SetDownloadProgress(fileHash, fileSize); err != nil {
		return err
	}

	//Verify downloaded file hash matches and remove the file if invalid
	err = c.storage.ValidateOrRemoveFile(fileHash)
	if err != nil {
		c.db.SetDownloadProgress(fileHash, 0)
	}
	return err
}
