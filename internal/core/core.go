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

func ClientMain(bind bool) {
	c := coreStartup()
	c.setupNotifyService(context.Background())

	if bind {
		var syncPhrase string
		fmt.Print("Enter sync phrase to bind to server: ")
		fmt.Scanln(&syncPhrase)
		c.bindNewServer(syncPhrase)
	}

	go c.p2pNode.ManageConnections(context.Background(), string(c.keys.MasterPublicKey))

	// Wait indefinitely
	select {}
}

func (c *Core) bindNewServer(syncPhrase string) {
	server, err := p2p.BindNewServer(syncPhrase, c.p2pNode, c.keys.MasterPublicKey)
	if err != nil {
		fmt.Println("Error binding to server:", err)
	}
	c.db.AddPeer(*server)
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
	p2pNode, err := p2p.NewP2PNode(keys.PeerPrivateKey, ctx, core.db.GetPeers())
	if err != nil {
		panic(fmt.Sprintf("Error starting P2P node: %v", err))
	}

	core.p2pNode = p2pNode
	core.keys = keys

	return core
}
