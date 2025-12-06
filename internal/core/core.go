package core

import (
	"context"
	"fmt"

	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"github.com/notassigned/endershare/internal/p2p"
)

func ClientMain() {
	//check for keys in db
	db := database.Create()

	keys := db.GetKeys()
	if keys == nil {
		var mnemonic string
		keys, mnemonic = crypto.CreateCryptoKeys()
		db.StoreKeys(keys)
		//output seed
		fmt.Println("Generated new keys with mnemonic:", mnemonic)
	}

	//start libp2p node with peer key
	//rendesvous on hash of master public key
	ctx := context.Background()
	p2pNode, err := p2p.StartP2PNode(keys.PeerPrivateKey, ctx)
	if err != nil {
		fmt.Println("Error starting P2P node:", err)
		return
	}

	p2pNode.EnableRoutingDiscovery(ctx, "test-rendezvous-point")

	//start sync loop, check for newer updates from remote
	//enable publishing after update check
}
