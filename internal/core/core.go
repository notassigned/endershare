package core

import (
	"context"
	"fmt"

	"github.com/notassigned/endershare/internal/crypto"
	"github.com/notassigned/endershare/internal/database"
	"github.com/notassigned/endershare/internal/p2p"
)

type Core struct {
	p2pNode *p2p.P2PNode
	keys    *crypto.CryptoKeys
	db      *database.EndershareDB
}

func ClientMain() {
	c := coreStartup()
	c.setupNotifyService(context.Background())
}

func coreStartup() *Core {
	core := &Core{
		db: database.Create(),
	}

	//Check for keys in db
	keys := core.db.GetKeys()
	if keys == nil {
		var mnemonic string
		keys, mnemonic = crypto.CreateCryptoKeys()
		core.db.StoreKeys(keys)
		//output seed
		fmt.Println("Generated new keys with mnemonic:", mnemonic)
	}

	ctx := context.Background()
	p2pNode, err := p2p.StartP2PNode(keys.PeerPrivateKey, ctx, core.db.GetPeers())
	if err != nil {
		panic(fmt.Sprintf("Error starting P2P node: %v", err))
	}

	core.p2pNode = p2pNode
	core.keys = keys
	go p2pNode.ManageConnections(ctx, string(keys.MasterPublicKey))
	return core
}
