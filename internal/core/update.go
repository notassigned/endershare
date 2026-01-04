package core

import (
	"crypto/ed25519"
	"encoding/json"
	"sort"

	"lukechampine.com/blake3"
)

type Update struct {
	UpdateID         uint64      `json:"update_id"`
	PeerListHash     []byte      `json:"peer_list_hash"`
	PrevPeerListHash []byte      `json:"prev_peer_list_hash"`
	DataHash         []byte      `json:"data_hash"`
	PrevDataHash     []byte      `json:"prev_data_hash"`
	NumBuckets       int         `json:"num_buckets"`
	UpdateDataType   string      `json:"update_data_type"` // "PEER" or "DATA"
	UpdateData       interface{} `json:"update_data"`
	Timestamp        int64       `json:"timestamp"`
}

type SignedUpdate struct {
	Update    Update `json:"update"`
	Signature []byte `json:"signature"`
}

type PeerUpdate struct {
	Action    string   `json:"action"`               // "ADD" or "REMOVE"
	PeerID    string   `json:"peer_id"`
	Addresses []string `json:"addresses,omitempty"` // Only for ADD
}

type DataUpdate struct {
	Action string `json:"action"` // "ADD", "MODIFY", "DELETE"
	Key    []byte `json:"key"`
	Value  []byte `json:"value,omitempty"` // File hash for files, nil for folders
	Size   int64  `json:"size,omitempty"`  // Size of file, 0 for folders
	Hash   []byte `json:"hash,omitempty"`  // For ADD/MODIFY, omitted for DELETE
}

// ComputePeerListHash creates a BLAKE3 hash of sorted peer IDs
func ComputePeerListHash(peerIDs []string) []byte {
	if len(peerIDs) == 0 {
		return make([]byte, 32) // Return zero hash for empty list
	}

	sorted := make([]string, len(peerIDs))
	copy(sorted, peerIDs)
	sort.Strings(sorted)

	hasher := blake3.New(32, nil)
	for _, peerID := range sorted {
		hasher.Write([]byte(peerID))
	}
	return hasher.Sum(nil)
}

// ComputeDataHash placeholder - returns zero hash until merkle tree is implemented
func ComputeDataHash(hashes [][]byte) []byte {
	// TODO: Implement merkle root computation
	return make([]byte, 32)
}

// VerifySignedUpdate verifies the signature over the Update JSON
func VerifySignedUpdate(signedUpdate SignedUpdate, publicKey ed25519.PublicKey) bool {
	updateJSON, err := json.Marshal(signedUpdate.Update)
	if err != nil {
		return false
	}
	return ed25519.Verify(publicKey, updateJSON, signedUpdate.Signature)
}
