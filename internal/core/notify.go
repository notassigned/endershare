package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (c *Core) setupNotifyService(ctx context.Context) error {
	return c.p2pNode.StartNotifyService(ctx, func(data []byte, from peer.ID) {
		buf := bytes.NewBuffer(data)
		msgType, err := buf.ReadString('\n')
		if err != nil {
			return
		}
		msgContent := buf.Bytes()

		switch strings.TrimSpace(msgType) {
		case "update":
			c.handleUpdate(msgContent, from)
		}
	})
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
