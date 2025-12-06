package p2p

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

// BindNewServer mutually verifies that the client and server both know the sync phrase
// We verify the server knows the phrase by checking that challenge = sha256(syncPhrase + peerID)
func BindNewServer(syncPhrase string, node P2PNode) (*peer.AddrInfo, error) {
	ctx := context.Background()
	nodes, err := node.EnableRoutingDiscovery(ctx, syncPhrase)

	if err != nil {
		return nil, err
	}
	for peerInfo := range nodes {
		fmt.Println("Found peer", peerInfo.ID)
		err := node.libp2pNode.Connect(ctx, peerInfo)
		if err != nil {
			fmt.Println("Error connecting to peer:", err)
			continue
		}
		stream, err := node.libp2pNode.NewStream(ctx, peerInfo.ID, "/endershare/bind/1.0")
		if err != nil {
			fmt.Println("Error creating stream to peer:", err)
			continue
		}

		ourProof := sha256.Sum256([]byte(syncPhrase + node.libp2pNode.ID().String()))
		stream.Write(ourProof[:])
		theirProof := make([]byte, 32)
		_, err = stream.Read(theirProof)
		if err != nil {
			fmt.Println("Error reading from stream:", err)
			continue
		}
		expectedProof := sha256.Sum256([]byte(syncPhrase + peerInfo.ID.String()))
		if !bytes.Equal(expectedProof[:], theirProof) {
			fmt.Println("Peer", peerInfo.ID, "failed verification")
			stream.Close()
			continue
		}

		return &peerInfo, nil
	}
	return nil, fmt.Errorf("no peers found")
}
