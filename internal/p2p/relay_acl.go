package p2p

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type RelayACL struct {
	p *P2PNode
}

func NewRelayACL(p *P2PNode) *RelayACL {
	return &RelayACL{p: p}
}

func (r *RelayACL) AllowReserve(p peer.ID, a multiaddr.Multiaddr) bool {
	return r.p.checkPeerAllowed(p)
}

func (r *RelayACL) AllowConnect(src peer.ID, srcAddr multiaddr.Multiaddr, dest peer.ID) bool {
	return r.p.checkPeerAllowed(src)
}
