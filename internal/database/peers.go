package database

import (
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func (db *EndershareDB) GetPeers() (peers []peer.AddrInfo) {
	rows, err := db.db.Query("SELECT peer_id, addresses FROM peers")
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
	_, err := db.db.Exec("INSERT OR REPLACE INTO peers (peer_id, addresses) VALUES (?, ?)", addrInfo.ID.String(), addressesStr)
	return err
}
