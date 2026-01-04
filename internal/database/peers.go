package database

import (
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type DBPeer struct {
	PeerID    string
	Addresses []string
}

func (db *EndershareDB) GetPeers() (peers []peer.AddrInfo) {
	rows, err := db.db.Query("SELECT peer_id, addrs FROM peers")
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var peerID string
		var addresses string
		if err := rows.Scan(&peerID, &addresses); err != nil {
			continue
		}
		//split addresses string by newlines
		p2pAddrs := []string{}
		multiaddrs := []multiaddr.Multiaddr{}
		for _, addr := range strings.Split(addresses, "\n") {
			p2pAddrs = append(p2pAddrs, addr)
		}
		for _, addr := range p2pAddrs {
			multiaddr, err := multiaddr.NewMultiaddr(addr)
			if err != nil {
				continue
			}
			multiaddrs = append(multiaddrs, multiaddr)
		}
		pID, err := peer.Decode(peerID)
		if err != nil {
			continue
		}
		addrInfo := &peer.AddrInfo{
			ID:    pID,
			Addrs: multiaddrs,
		}

		peers = append(peers, *addrInfo)
	}
	return peers
}

func (db *EndershareDB) AddPeer(addrInfo peer.AddrInfo) error {
	addresses := []string{}
	for _, addr := range addrInfo.Addrs {
		addresses = append(addresses, addr.String())
	}
	addressesStr := strings.Join(addresses, "\n")
	_, err := db.db.Exec("INSERT OR REPLACE INTO peers (peer_id, addrs) VALUES (?, ?)", addrInfo.ID.String(), addressesStr)
	return err
}

// GetAllPeerIDs returns a sorted list of all peer IDs
func (db *EndershareDB) GetAllPeerIDs() []string {
	rows, err := db.db.Query("SELECT peer_id FROM peers ORDER BY peer_id")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var peerIDs []string
	for rows.Next() {
		var peerID string
		if err := rows.Scan(&peerID); err != nil {
			continue
		}
		peerIDs = append(peerIDs, peerID)
	}
	return peerIDs
}

// UpdatePeerAddresses updates the addresses for an existing peer
func (db *EndershareDB) UpdatePeerAddresses(peerID string, addrs []string) error {
	addressesStr := strings.Join(addrs, "\n")
	_, err := db.db.Exec("UPDATE peers SET addrs = ? WHERE peer_id = ?", addressesStr, peerID)
	return err
}

// RemovePeer removes a peer by ID
func (db *EndershareDB) RemovePeer(peerID string) error {
	_, err := db.db.Exec("DELETE FROM peers WHERE peer_id = ?", peerID)
	return err
}

// ReplaceAllPeers atomically replaces all peers with a new list
func (db *EndershareDB) ReplaceAllPeers(peers []DBPeer) error {
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete all existing peers
	_, err = tx.Exec("DELETE FROM peers")
	if err != nil {
		return err
	}

	// Insert all new peers
	stmt, err := tx.Prepare("INSERT INTO peers (peer_id, addrs) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, peer := range peers {
		addressesStr := strings.Join(peer.Addresses, "\n")
		_, err = stmt.Exec(peer.PeerID, addressesStr)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
