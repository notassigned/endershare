package p2p

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
)

type P2PNode struct {
	libp2pNode host.Host
}

func StartP2PNode(peerPrivKey ed25519.PrivateKey, ctx context.Context) (*P2PNode, error) {
	lpriv, err := crypto.UnmarshalEd25519PrivateKey(peerPrivKey)
	if err != nil {
		return nil, err
	}
	host, err := libp2p.New(
		libp2p.Identity(lpriv),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.EnableRelayService(),
		libp2p.DisableMetrics(),
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/13000",
			"/ip6/::/tcp/13000",
			"/ip4/0.0.0.0/udp/13000/quic",
			"/ip6/::/udp/13000/quic"),
	)

	if err != nil {
		return nil, err
	}

	return &P2PNode{libp2pNode: host}, nil
}

func (p *P2PNode) EnableRoutingDiscovery(ctx context.Context, rendesvous string) (<-chan peer.AddrInfo, error) {
	//setup discovery using the kademlia DHT
	kademliaDHT, err := dht.New(ctx, p.libp2pNode)
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(append([]byte("endershare-rendezvous"), []byte(rendesvous)...))

	err = kademliaDHT.Bootstrap(ctx)

	if err != nil {
		return nil, err
	}

	routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)

	peers, err := routingDiscovery.FindPeers(ctx, string(key[:]), discovery.TTL(time.Hour))
	if err != nil {
		return nil, err
	}

	return peers, nil
}
