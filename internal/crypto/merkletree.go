package crypto

import (
	"bytes"
	"math/big"

	"lukechampine.com/blake3"
)

const HASHES_PER_BUCKET = 10

type MerkleNode struct {
	Hash  []byte
	Left  *MerkleNode
	Right *MerkleNode
}

type Bucket struct {
	Hashes [][]byte
}

type MerkleTree struct {
	Buckets    []*Bucket
	NumBuckets int
	Root       *MerkleNode
}

// ComputeHash computes the BLAKE3 hash of the bucket's contents
func (b *Bucket) ComputeHash() []byte {
	if len(b.Hashes) == 0 {
		return make([]byte, 32)
	}

	// Concatenate all hashes in the bucket (already sorted by value)
	var buf bytes.Buffer
	for _, h := range b.Hashes {
		buf.Write(h)
	}

	hasher := blake3.New(32, nil)
	hasher.Write(buf.Bytes())
	return hasher.Sum(nil)
}

// calculateNumBuckets determines optimal number of buckets for given hash count
func calculateNumBuckets(numHashes int) int {
	if numHashes == 0 {
		return 1
	}
	return (numHashes + HASHES_PER_BUCKET - 1) / HASHES_PER_BUCKET
}

// getBucketIndex determines which bucket a hash belongs to based on its value
// Divides the 256-bit hash space into numBuckets ranges
func getBucketIndex(hash []byte, numBuckets int) int {
	if numBuckets <= 1 {
		return 0
	}

	// Convert hash to big.Int
	hashInt := new(big.Int).SetBytes(hash)

	// Calculate bucket size: 2^256 / numBuckets
	maxHash := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256
	bucketSize := new(big.Int).Div(maxHash, big.NewInt(int64(numBuckets)))

	// Determine which bucket: floor(hashInt / bucketSize)
	bucketIdx := new(big.Int).Div(hashInt, bucketSize)

	idx := int(bucketIdx.Int64())
	if idx >= numBuckets {
		idx = numBuckets - 1
	}
	return idx
}

// NewMerkleTree creates a new Merkle tree from a list of hashes
// Hashes are distributed into buckets based on their value ranges
func NewMerkleTree(hashes [][]byte) *MerkleTree {
	numBuckets := calculateNumBuckets(len(hashes))
	return newMerkleTreeWithBuckets(hashes, numBuckets)
}

// newMerkleTreeWithBuckets creates a tree with a specific number of buckets
func newMerkleTreeWithBuckets(hashes [][]byte, numBuckets int) *MerkleTree {
	if numBuckets < 1 {
		numBuckets = 1
	}

	buckets := make([]*Bucket, numBuckets)
	for i := 0; i < numBuckets; i++ {
		buckets[i] = &Bucket{Hashes: [][]byte{}}
	}

	// Distribute hashes into buckets based on their value
	for _, hash := range hashes {
		bucketIdx := getBucketIndex(hash, numBuckets)
		buckets[bucketIdx].Hashes = append(buckets[bucketIdx].Hashes, hash)
	}

	// Sort hashes within each bucket
	for _, bucket := range buckets {
		sortHashes(bucket.Hashes)
	}

	// Build tree from buckets
	root := buildTree(buckets)

	return &MerkleTree{
		Buckets:    buckets,
		NumBuckets: numBuckets,
		Root:       root,
	}
}

// sortHashes sorts a slice of hashes by their byte values
func sortHashes(hashes [][]byte) {
	// Simple insertion sort for small slices
	for i := 1; i < len(hashes); i++ {
		j := i
		for j > 0 && bytes.Compare(hashes[j-1], hashes[j]) > 0 {
			hashes[j-1], hashes[j] = hashes[j], hashes[j-1]
			j--
		}
	}
}

// buildTree recursively builds the Merkle tree from buckets
func buildTree(buckets []*Bucket) *MerkleNode {
	if len(buckets) == 0 {
		return nil
	}

	// Create leaf nodes from buckets
	nodes := make([]*MerkleNode, len(buckets))
	for i, bucket := range buckets {
		nodes[i] = &MerkleNode{
			Hash: bucket.ComputeHash(),
		}
	}

	// Build tree bottom-up
	return buildTreeFromNodes(nodes)
}

// buildTreeFromNodes recursively builds tree from nodes
func buildTreeFromNodes(nodes []*MerkleNode) *MerkleNode {
	if len(nodes) == 0 {
		return nil
	}
	if len(nodes) == 1 {
		return nodes[0]
	}

	// Build parent level
	var parentLevel []*MerkleNode
	for i := 0; i < len(nodes); i += 2 {
		left := nodes[i]
		var right *MerkleNode
		if i+1 < len(nodes) {
			right = nodes[i+1]
		}

		// Compute parent hash from children
		hasher := blake3.New(32, nil)
		hasher.Write(left.Hash)
		if right != nil {
			hasher.Write(right.Hash)
		}

		parent := &MerkleNode{
			Hash:  hasher.Sum(nil),
			Left:  left,
			Right: right,
		}
		parentLevel = append(parentLevel, parent)
	}

	return buildTreeFromNodes(parentLevel)
}

// GetRootHash returns the root hash of the tree
func (mt *MerkleTree) GetRootHash() []byte {
	if mt.Root == nil {
		return make([]byte, 32)
	}
	return mt.Root.Hash
}

// GetBucketHashes returns all bucket hashes in order
func (mt *MerkleTree) GetBucketHashes() [][]byte {
	hashes := make([][]byte, len(mt.Buckets))
	for i, bucket := range mt.Buckets {
		hashes[i] = bucket.ComputeHash()
	}
	return hashes
}

// GetBucketData returns the hashes in a specific bucket
func (mt *MerkleTree) GetBucketData(bucketIdx int) [][]byte {
	if bucketIdx < 0 || bucketIdx >= len(mt.Buckets) {
		return nil
	}
	return mt.Buckets[bucketIdx].Hashes
}

// Insert adds a new hash to the tree
// Returns true if tree was rebuilt with more buckets
func (mt *MerkleTree) Insert(hash []byte) bool {
	// Find which bucket this hash belongs to
	bucketIdx := getBucketIndex(hash, mt.NumBuckets)

	// Insert into bucket (maintaining sorted order)
	bucket := mt.Buckets[bucketIdx]
	insertPos := 0
	for i, h := range bucket.Hashes {
		if bytes.Equal(h, hash) {
			// Hash already exists, no-op
			return false
		}
		if bytes.Compare(hash, h) < 0 {
			insertPos = i
			break
		}
		insertPos = i + 1
	}

	// Insert at position
	bucket.Hashes = append(bucket.Hashes[:insertPos], append([][]byte{hash}, bucket.Hashes[insertPos:]...)...)

	// Check if we need to rebuild with more buckets
	totalHashes := mt.getTotalHashes()
	avgPerBucket := float64(totalHashes) / float64(mt.NumBuckets)

	if avgPerBucket > float64(HASHES_PER_BUCKET*2) {
		// Rebuild with more buckets
		mt.rebuild()
		return true
	}

	// Just rebuild the tree structure (not the buckets)
	mt.Root = buildTree(mt.Buckets)
	return false
}

// Delete removes a hash from the tree
// Returns true if tree was rebuilt with fewer buckets
func (mt *MerkleTree) Delete(hash []byte) bool {
	// Find which bucket this hash belongs to
	bucketIdx := getBucketIndex(hash, mt.NumBuckets)

	// Remove from bucket
	bucket := mt.Buckets[bucketIdx]
	for i, h := range bucket.Hashes {
		if bytes.Equal(h, hash) {
			bucket.Hashes = append(bucket.Hashes[:i], bucket.Hashes[i+1:]...)
			break
		}
	}

	// Check if we need to rebuild with fewer buckets
	totalHashes := mt.getTotalHashes()
	if totalHashes > 0 {
		avgPerBucket := float64(totalHashes) / float64(mt.NumBuckets)

		if avgPerBucket < float64(HASHES_PER_BUCKET)/4 && mt.NumBuckets > 1 {
			// Rebuild with fewer buckets
			mt.rebuild()
			return true
		}
	}

	// Just rebuild the tree structure (not the buckets)
	mt.Root = buildTree(mt.Buckets)
	return false
}

// getTotalHashes counts all hashes across all buckets
func (mt *MerkleTree) getTotalHashes() int {
	total := 0
	for _, bucket := range mt.Buckets {
		total += len(bucket.Hashes)
	}
	return total
}

// rebuild collects all hashes and redistributes them into new buckets
func (mt *MerkleTree) rebuild() {
	// Collect all hashes
	allHashes := [][]byte{}
	for _, bucket := range mt.Buckets {
		allHashes = append(allHashes, bucket.Hashes...)
	}

	// Calculate new number of buckets
	newNumBuckets := calculateNumBuckets(len(allHashes))

	// Rebuild tree with new bucket count
	newTree := newMerkleTreeWithBuckets(allHashes, newNumBuckets)
	mt.Buckets = newTree.Buckets
	mt.NumBuckets = newTree.NumBuckets
	mt.Root = newTree.Root
}

// DiffBuckets compares this tree with another and returns indices of buckets that differ
// This is used for efficient syncing between nodes
func (mt *MerkleTree) DiffBuckets(other *MerkleTree) []int {
	if other == nil {
		return nil
	}

	myBuckets := mt.GetBucketHashes()
	otherBuckets := other.GetBucketHashes()

	// If trees have different number of buckets, they need full sync
	if len(myBuckets) != len(otherBuckets) {
		var all []int
		maxLen := len(myBuckets)
		if len(otherBuckets) > maxLen {
			maxLen = len(otherBuckets)
		}
		for i := 0; i < maxLen; i++ {
			all = append(all, i)
		}
		return all
	}

	var diff []int
	for i := 0; i < len(myBuckets); i++ {
		// Compare bucket hashes
		if !bytes.Equal(myBuckets[i], otherBuckets[i]) {
			diff = append(diff, i)
		}
	}

	return diff
}

// GetNumBuckets returns the number of buckets in the tree
func (mt *MerkleTree) GetNumBuckets() int {
	return mt.NumBuckets
}
