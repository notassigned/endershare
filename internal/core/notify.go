package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (c *Core) setupNotifyService(ctx context.Context) error {
	mtx := &sync.Mutex{}
	publishNotification, err := c.p2pNode.StartNotifyService(ctx, func(data []byte, from peer.ID) {
		mtx.Lock()
		defer mtx.Unlock()

		// Parse message type
		buf := bytes.NewBuffer(data)
		msgType, err := buf.ReadString('\n')
		if err != nil {
			return
		}
		msgContent := buf.Bytes()
		fmt.Println("Recvd", msgType, ":", string(msgContent))

		switch strings.TrimSpace(msgType) {
		case "update":
			c.handleUpdate(msgContent, from)
		case "request_latest_update":
			c.handleLatestUpdateRequest()
		default:
			return
		}
	}, c.keys.MasterPublicKey[:32])
	c.publishUpdate = publishNotification
	return err
}

// Notify sends a message to all peers via gossipsub
func (c *Core) notify(msgType string, msg []byte) error {
	if c.publishUpdate == nil {
		return fmt.Errorf("notify service not initialized")
	}
	return c.publishUpdate(append([]byte(msgType+"\n"), msg...))
}

func (c *Core) handleUpdate(notification []byte, from peer.ID) {
	var signedUpdate SignedUpdate
	if err := json.Unmarshal(notification, &signedUpdate); err != nil {
		fmt.Println("Failed to unmarshal update notification:", err)
		return
	}

	// Sync logic is implemented in sync.go
	if err := c.processUpdate(signedUpdate, from); err != nil {
		fmt.Println("Failed to process update:", err)
	}
}

func (c *Core) handleLatestUpdateRequest() {
	latest, err := c.db.GetNodeProperty("lastest_update")
	if err != nil {
		return
	}
	c.notify("update", []byte(latest))
}
