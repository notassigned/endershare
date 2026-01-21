package p2p

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	gossipsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/notassigned/endershare/internal/safemap"
	"golang.org/x/crypto/scrypt"
)

type P2PNode struct {
	host        host.Host
	notifyTopic *gossipsub.Topic
	peers       *safemap.SafeMap[peer.ID, peer.AddrInfo]
	dht         *dht.IpfsDHT
	discovery   *routing.RoutingDiscovery
}

func NewP2PNode(peerPrivKey ed25519.PrivateKey, ctx context.Context, peers []peer.AddrInfo, port int) (*P2PNode, error) {
	n := &P2PNode{
		peers: safemap.NewSafeMap[peer.ID, peer.AddrInfo](),
	}
	for _, p := range peers {
		n.peers.Store(p.ID, p)
	}

	// Convert ed25519 private key to libp2p crypto.PrivateKey
	lpriv, err := crypto.UnmarshalEd25519PrivateKey(peerPrivKey)
	if err != nil {
		return nil, err
	}
	host, err := libp2p.New(
		libp2p.Identity(lpriv),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.EnableRelayService(relay.WithACL(NewRelayACL(n)), relay.WithInfiniteLimits()),
		libp2p.DisableMetrics(),
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
			fmt.Sprintf("/ip6/::/tcp/%d", port),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", port),
			fmt.Sprintf("/ip6/::/udp/%d/quic", port),
		),
	)

	if err != nil {
		return nil, err
	}
	fmt.Println("Node started with ID:", host.ID())

	n.host = host

	err = n.setupDiscovery(ctx)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func (p *P2PNode) GetPeerId() peer.ID {
	return p.host.ID()
}

func (p *P2PNode) NewStreamToPeer(peerID peer.ID, protocolID string) (network.Stream, error) {
	if p.checkPeerAllowed(peerID) {
		return nil, fmt.Errorf("peer not allowed")
	}
	stream, err := p.host.NewStream(context.Background(), peerID, protocol.ID(protocolID))
	return stream, err
}

// NewStreamHandler sets a stream handler for authenticated peers
func (p *P2PNode) NewStreamHandler(protocolID string, handler func(network.Stream)) {
	p.host.SetStreamHandler(protocol.ID(protocolID), func(s network.Stream) {
		if !p.checkPeerAllowed(s.Conn().RemotePeer()) {
			s.Reset()
			return
		}
		handler(s)
	})
}

func (p *P2PNode) AddPeer(addrInfo peer.AddrInfo) {
	p.peers.Store(addrInfo.ID, addrInfo)
}

func (p *P2PNode) ReplacePeers(peers []peer.AddrInfo) {
	p.peers.Clear()
	for _, peerInfo := range peers {
		p.peers.Store(peerInfo.ID, peerInfo)
	}
}

func (p *P2PNode) setupDiscovery(ctx context.Context) error {
	//setup discovery using the kademlia DHT
	kademliaDHT, err := dht.New(ctx, p.host, dht.BootstrapPeers(dht.GetDefaultBootstrapPeerAddrInfos()...))
	if err != nil {
		return err
	}

	err = kademliaDHT.Bootstrap(ctx)

	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.host.Connect(ctx, *peerinfo); err != nil {
				fmt.Println("Bootstrap warning:", err)
			}
		}()
	}

	wg.Wait()

	routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)

	p.dht = kademliaDHT
	p.discovery = routingDiscovery

	return nil
}

func (p *P2PNode) discoverPeers(ctx context.Context, rendesvous string) (<-chan peer.AddrInfo, error) {
	key, err := scrypt.Key([]byte(rendesvous), []byte("endershare-rendezvous"), 32768, 8, 1, 32)
	if err != nil {
		return nil, err
	}
	peers, err := p.discovery.FindPeers(ctx, string(key[:]), discovery.TTL(time.Hour))
	if err != nil {
		return nil, err
	}

	return peers, nil
}

func (p *P2PNode) Advertize(ctx context.Context, rendesvous string, ttl time.Duration) error {
	key, err := scrypt.Key([]byte(rendesvous), []byte("endershare-rendezvous"), 32768, 8, 1, 32)
	if err != nil {
		return err
	}
	opts := []discovery.Option{}
	if ttl != 0 {
		opts = append(opts, discovery.TTL(ttl))
	}
	_, err = p.discovery.Advertise(ctx, string(key[:]), opts...)
	return err
}

func (p *P2PNode) ManageConnections(ctx context.Context, key string) {
	// Advertize ourselves
	err := p.Advertize(ctx, key, 0)
	if err != nil {
		fmt.Println("Error advertising:", err)
	}

	peers, err := p.discoverPeers(ctx, key)
	if err != nil {
		fmt.Println("Error enabling discovery:", err)
		return
	}
	for {
		select {
		case peer := <-peers:
			if p.checkPeerAllowed(peer.ID) {
				p.host.Connect(ctx, peer)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *P2PNode) checkPeerAllowed(peerID peer.ID) bool {
	_, exists := p.peers.Load(peerID)
	return exists
}

// GetPeerStatus returns whether a peer is currently connected and when it was last seen
func (p *P2PNode) GetPeerStatus(peerIDStr string) (isOnline bool, lastSeen time.Time) {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return false, time.Time{}
	}

	// Check if peer is currently connected
	conns := p.host.Network().ConnsToPeer(peerID)
	isOnline = len(conns) > 0

	// For now, we don't track last seen time - would need to add connection event tracking
	// Return current time if online, zero time if not
	if isOnline {
		lastSeen = time.Now()
	}

	return isOnline, lastSeen
}
