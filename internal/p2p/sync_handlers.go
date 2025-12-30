package p2p

import (
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/notassigned/endershare/internal/database"
)

type PeerInfoResponse struct {
	PeerID        string   `json:"peer_id"`
	Addresses     []string `json:"addresses"`
	PeerSignature []byte   `json:"peer_signature"`
}

// SetupSyncHandlers registers stream handlers for syncing
func (p *P2PNode) SetupSyncHandlers(db *database.EndershareDB) {
	p.host.SetStreamHandler(protocol.ID("/endershare/peer-list/1.0"), func(s network.Stream) {
		handlePeerListRequest(s, db)
	})
}

// handlePeerListRequest handles requests for the full peer list
func handlePeerListRequest(s network.Stream, db *database.EndershareDB) {
	defer s.Close()

	// Get all peers from database
	peers := db.GetPeers()

	// Convert to response format
	response := []PeerInfoResponse{}
	for _, peerInfo := range peers {
		addrs := []string{}
		for _, addr := range peerInfo.Addrs {
			addrs = append(addrs, addr.String())
		}

		// Get peer signature from database
		peerSignature := []byte{} // TODO: Store and retrieve peer signature
		response = append(response, PeerInfoResponse{
			PeerID:        peerInfo.ID.String(),
			Addresses:     addrs,
			PeerSignature: peerSignature,
		})
	}

	// Send response
	encoder := json.NewEncoder(s)
	encoder.Encode(response)
}
