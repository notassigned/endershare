package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"

	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/scrypt"
	"lukechampine.com/blake3"
)

type CryptoKeys struct {
	MasterPrivateKey ed25519.PrivateKey
	MasterPublicKey  ed25519.PublicKey
	PeerPrivateKey   ed25519.PrivateKey
	PeerPublicKey    ed25519.PublicKey
	PeerSignature    []byte
	AESKey           []byte
}

func (c *CryptoKeys) MasterSign(message []byte) []byte {
	signature := ed25519.Sign(c.MasterPrivateKey, message)
	return signature
}

// this function creates a mnemonic and derives an ed25519 keypair and AES-256 key from it
func CreateCryptoKeys() (*CryptoKeys, string) {
	entropy, _ := bip39.NewEntropy(256)
	m, _ := bip39.NewMnemonic(entropy)
	return SetupKeysFromMnemonic(m), m
}

func VerifySignature(publicKey ed25519.PublicKey, message []byte, signature []byte) bool {
	ok := ed25519.Verify(publicKey, message, signature)
	return ok
}

func NewCryptoKeysFromBytes(masterPriv []byte, peerPriv []byte, aesKey []byte) *CryptoKeys {
	masterPrivateKey := ed25519.PrivateKey(masterPriv)
	peerPrivateKey := ed25519.PrivateKey(peerPriv)

	return &CryptoKeys{
		MasterPrivateKey: masterPrivateKey,
		MasterPublicKey:  masterPrivateKey.Public().(ed25519.PublicKey),
		PeerPrivateKey:   peerPrivateKey,
		PeerPublicKey:    peerPrivateKey.Public().(ed25519.PublicKey),
		AESKey:           aesKey,
	}
}

func SetupKeysFromMnemonic(mnemonic string) *CryptoKeys {
	key, err := scrypt.Key([]byte(mnemonic), []byte("endershare"), 32768, 8, 1, 32)
	if err != nil {
		panic(err)
	}

	//generate master ed25519 keypair
	priv := ed25519.NewKeyFromSeed(key)
	pub := priv.Public().(ed25519.PublicKey)

	//generate peer ed25519 keypair
	randPeerSeed := make([]byte, 32)
	rand.Read(randPeerSeed)
	peerPriv := ed25519.NewKeyFromSeed(randPeerSeed)
	peerPub := peerPriv.Public().(ed25519.PublicKey)

	//sign peer public key with master private key
	peerSignature := ed25519.Sign(priv, peerPub)

	//generate encryption key
	AESKey := sha256.Sum256(key)

	return &CryptoKeys{
		MasterPrivateKey: priv,
		MasterPublicKey:  pub,
		PeerPrivateKey:   peerPriv,
		PeerPublicKey:    peerPub,
		PeerSignature:    peerSignature,
		AESKey:           AESKey[:],
	}
}

func ComputeDataHash(data []byte) []byte {
	h := blake3.New(len(data), data)
	return h.Sum(nil)
}
