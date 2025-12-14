package core

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"

	"github.com/notassigned/endershare/internal/database"
	"github.com/notassigned/endershare/internal/p2p"
)

func ServerMain() {
	core := coreStartup()

	if getMasterPubKey(core.db) == nil {
		core.bindToClient()
	}
}

func (core *Core) bindToClient() {
	client, err := p2p.BindToClient(core.p2pNode)
	if err != nil {
		panic(fmt.Sprintf("Error binding to client: %v", err))
	}
	err = core.db.SetNodeProperty("master_public_key", base64.StdEncoding.EncodeToString(client.MasterPublicKey))
	if err != nil {
		panic(fmt.Sprintf("Error storing master public key: %v", err))
	}
	err = core.db.AddPeer(client.AddrInfo, client.PeerSignature)
	if err != nil {
		panic(fmt.Sprintf("Error adding peer: %v", err))
	}

	core.keys.MasterPublicKey = client.MasterPublicKey
	fmt.Println("Successfully bound to client:", client.PeerID)
}

func getMasterPubKey(db *database.EndershareDB) ed25519.PublicKey {
	k, err := db.GetMasterPubKey()
	if err != nil {
		return nil
	}
	return k
}
