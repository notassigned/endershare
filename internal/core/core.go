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

func ServerMain() {
	db := database.Create()

	//only the peer key will be used for p2p identity
	//the master public key will be filled in later once linked to a client
	keys := db.GetKeys()
	if keys == nil {
		keys, _ = crypto.CreateCryptoKeys()
		db.StoreKeys(keys)
		db.DeleteNodeProperty("master_public_key")
		db.DeleteNodeProperty("master_private_key")
	}

	//read in sync phrase from user

	var syncPhrase string
	fmt.Print("Enter sync phrase: ")
	fmt.Scanln(&syncPhrase)

	p2pNode, err := p2p.StartP2PNode(keys.PeerPrivateKey, context.Background())
	if err != nil {
		fmt.Println("Error starting P2P node:", err)
		return
	}

	p2pNode.EnableRoutingDiscovery(context.Background(), syncPhrase)
}
