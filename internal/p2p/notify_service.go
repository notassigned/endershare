package p2p

import (
	"context"

	gossipsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (p *P2PNode) StartNotifyService(ctx context.Context, notification func([]byte, []byte)) error {
	gossip, err := gossipsub.NewGossipSub(ctx, p.host, gossipsub.WithPeerFilter(p.filterNotifyPeers))
	if err != nil {
		return err
	}

	topic, err := gossip.Join("endershare-1.0")
	if err != nil {
		return err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}

	go func() {
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				return
			}

			notification(msg.Message.Data, msg.Message.From)
		}
	}()
	p.notifyTopic = topic
	return nil
}

func (p *P2PNode) PublishNotification(data []byte) error {
	return p.notifyTopic.Publish(context.Background(), data)
}

func (p *P2PNode) filterNotifyPeers(peerID peer.ID, topic string) bool {
	if _, ok := p.peers[peerID]; ok {
		return true
	}
	return false
}
