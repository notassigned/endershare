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
		bindToClient(core.db, core.p2pNode)
	}
}

func bindToClient(db *database.EndershareDB, p2pNode *p2p.P2PNode) {
	client, err := p2p.BindNewClient(*p2pNode)
	if err != nil {
		panic(fmt.Sprintf("Error binding to client: %v", err))
	}
	err = db.SetNodeProperty("master_public_key", base64.StdEncoding.EncodeToString(client.MasterPublicKey))
	if err != nil {
		panic(fmt.Sprintf("Error storing master public key: %v", err))
	}

}

func getMasterPubKey(db *database.EndershareDB) ed25519.PublicKey {
	k, err := db.GetMasterPubKey()
	if err != nil {
		return nil
	}
	return k
}
