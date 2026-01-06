package p2p

import (
	"context"
	"encoding/hex"

	gossipsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (p *P2PNode) StartNotifyService(ctx context.Context, notification func([]byte, peer.ID), topicID []byte) (publishNotification func([]byte) error, err error) {
	gossip, err := gossipsub.NewGossipSub(ctx,
		p.host,
		gossipsub.WithPeerFilter(p.filterNotifyPeers),
		gossipsub.WithDiscovery(p.discovery))
	if err != nil {
		return nil, err
	}

	topic, err := gossip.Join(hex.EncodeToString(topicID))
	if err != nil {
		return nil, err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				return
			}

			notification(msg.Message.Data, msg.ReceivedFrom)
		}
	}()
	p.notifyTopic = topic
	return func(data []byte) error {
		return p.notifyTopic.Publish(context.Background(), data)
	}, nil
}

func (p *P2PNode) filterNotifyPeers(peerID peer.ID, topic string) bool {
	if _, ok := p.peers.Load(peerID); ok {
		return true
	}
	return false
}
