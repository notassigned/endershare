package database

import (
	"crypto/ed25519"
	"encoding/base64"
	"log"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/notassigned/endershare/internal/crypto"
)

func (db *EndershareDB) GetMasterPubKey() (ed25519.PublicKey, error) {
	key, err := db.GetNodeProperty("master_public_key")
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(decoded), nil
}

func (db *EndershareDB) GetKeys() *crypto.CryptoKeys {
	rows, err := db.db.Query("SELECT key, value FROM node WHERE key IN ('master_private_key', 'peer_private_key', 'aes_key')")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	keys := make(map[string]string)
	count := 0
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			log.Fatal(err)
		}
		keys[key] = value
		count++
	}

	if count < 3 || keys["master_private_key"] == "" || keys["peer_private_key"] == "" || keys["aes_key"] == "" {
		return nil
	}

	mpriv, err := base64.StdEncoding.DecodeString(keys["master_private_key"])
	if err != nil {
		log.Fatal(err)
	}
	ppriv, err := base64.StdEncoding.DecodeString(keys["peer_private_key"])
	if err != nil {
		log.Fatal(err)
	}
	aesKey, err := base64.StdEncoding.DecodeString(keys["aes_key"])
	if err != nil {
		log.Fatal(err)
	}

	return crypto.NewCryptoKeysFromBytes(mpriv, ppriv, aesKey)
}

// StoreKeys saves the master private key, peer private key, and AES key into the database
// StoreKeys also inserts the peer's public key into the peers table
func (db *EndershareDB) StoreKeys(keys *crypto.CryptoKeys) {
	masterPrivEnc := base64.StdEncoding.EncodeToString(keys.MasterPrivateKey)
	peerPrivEnc := base64.StdEncoding.EncodeToString(keys.PeerPrivateKey)
	aesKeyEnc := base64.StdEncoding.EncodeToString(keys.AESKey)

	insertStmt := `
	INSERT OR REPLACE INTO node (key, value) VALUES
		('master_private_key', ?),
		('peer_private_key', ?),
		('aes_key', ?);
	`
	_, err := db.db.Exec(insertStmt, masterPrivEnc, peerPrivEnc, aesKeyEnc)
	if err != nil {
		log.Fatal(err)
	}

	// Store master public key
	if keys.MasterPublicKey != nil {
		masterPubEnc := base64.StdEncoding.EncodeToString(keys.MasterPublicKey)
		err = db.SetNodeProperty("master_public_key", masterPubEnc)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Store peer in peers table
	lpriv, err := libp2pcrypto.UnmarshalEd25519PrivateKey(keys.PeerPrivateKey)
	if err != nil {
		log.Fatal(err)
	}

	peerID, err := peer.IDFromPrivateKey(lpriv)
	if err != nil {
		log.Fatal(err)
	}

	addrInfo := peer.AddrInfo{
		ID:    peerID,
		Addrs: []multiaddr.Multiaddr{},
	}

	err = db.AddPeer(addrInfo)
	if err != nil {
		log.Printf("Warning: Failed to add peer to database: %v", err)
	}
}
