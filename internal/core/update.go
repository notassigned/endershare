package core

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
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
	UpdateBytes []byte `json:"update_bytes"` // Canonical JSON bytes of the update
	Signature   []byte `json:"signature"`
}

type PeerUpdate struct {
	Action    string   `json:"action"` // "ADD" or "REMOVE"
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

// VerifySignedUpdate verifies the signature over the canonical update bytes
func VerifySignedUpdate(signedUpdate SignedUpdate, publicKey ed25519.PublicKey) bool {
	return ed25519.Verify(publicKey, signedUpdate.UpdateBytes, signedUpdate.Signature)
}

// GetUpdate unmarshals the Update from SignedUpdate.UpdateBytes
func (s *SignedUpdate) GetUpdate() (Update, error) {
	var update Update
	err := json.Unmarshal(s.UpdateBytes, &update)
	return update, err
}

func SignUpdate(update Update, privateKey ed25519.PrivateKey) (SignedUpdate, error) {
	updateJSON, err := json.Marshal(update)
	if err != nil {
		return SignedUpdate{}, fmt.Errorf("failed to marshal update: %w", err)
	}
	signature := ed25519.Sign(privateKey, updateJSON)
	return SignedUpdate{
		UpdateBytes: updateJSON,
		Signature:   signature,
	}, nil
}
