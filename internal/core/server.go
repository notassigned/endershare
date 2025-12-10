package core

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"github.com/notassigned/endershare/internal/p2p"
)

func ServerMain() {
	db := database.Create()

	//only the peer key will be used for p2p identity
	//the master public key will be filled in later once linked to a client
	keys := db.GetKeys()
	if keys == nil {
		keys, _ = crypto.CreateCryptoKeys()
		db.StoreKeys(keys)
	}

	p2pNode, err := p2p.StartP2PNode(keys.PeerPrivateKey, context.Background())
	if err != nil {
		fmt.Println("Error starting P2P node:", err)
		return
	}

	if !boundToClient(db) {
		bindToClient(db, p2pNode)
	}
	//read in sync phrase from user

	var syncPhrase string
	fmt.Print("Enter sync phrase: ")
	fmt.Scanln(&syncPhrase)

	p2pNode.EnableRoutingDiscovery(context.Background(), syncPhrase)
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

func boundToClient(db *database.EndershareDB) bool {
	_, err := db.GetMasterPubKey()
	return err == nil
}
