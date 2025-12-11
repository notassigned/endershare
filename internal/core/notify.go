package core

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
)

type DataUpdateNotification struct {
	UpdateID  uint64
	RootHash  []byte
	Signature []byte
}

func (c *Core) setupNotifyService(ctx context.Context) error {
	return c.p2pNode.StartNotifyService(ctx, func(data []byte, from []byte) {
		buf := bytes.NewBuffer(data)
		msgType, err := buf.ReadString('\n')
		if err != nil {
			return
		}
		msgContent := buf.Bytes()

		switch strings.TrimSpace(msgType) {
		case "data_update":
			c.handleDataUpdate(msgContent)
		}
	})
}

func (c *Core) handleDataUpdate(notification []byte) {
	var dataUpdate DataUpdateNotification
	if json.Unmarshal(notification, &dataUpdate) != nil {
		return
	}

	//here is where we will handle the request logic for data updates
}
