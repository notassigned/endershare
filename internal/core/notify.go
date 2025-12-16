package core

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"strings"
)

type DataUpdateNotification struct {
	UpdateID  uint64
	RootHash  []byte
	FromPeer  []byte
	Signature []byte // Signature of (UpdateID+RootHash+FromPeer) by the master key
}

func signUpdateNotification(update DataUpdateNotification, privateKey ed25519.PrivateKey) ([]byte, error) {
	dataToSign := append([]byte(fmt.Sprintf("%d", update.UpdateID)), update.RootHash...)
	dataToSign = append(dataToSign, update.FromPeer...)
	signature := ed25519.Sign(privateKey, dataToSign)
	return signature, nil
}

func verifyUpdateNotification(update DataUpdateNotification, publicKey ed25519.PublicKey) bool {
	dataToVerify := append([]byte(fmt.Sprintf("%d", update.UpdateID)), update.RootHash...)
	dataToVerify = append(dataToVerify, update.FromPeer...)
	return ed25519.Verify(publicKey, dataToVerify, update.Signature)
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
	if err := json.Unmarshal(notification, &dataUpdate); err != nil {
		fmt.Println("Failed to unmarshal data update notification:", err)
		return
	}

	//here is where we will handle the request logic for data updates
}
