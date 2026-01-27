package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/scrypt"
	"lukechampine.com/blake3"
)

type CryptoKeys struct {
	MasterPrivateKey ed25519.PrivateKey
	MasterPublicKey  ed25519.PublicKey
	PeerPrivateKey   ed25519.PrivateKey
	PeerPublicKey    ed25519.PublicKey
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

// CreatePeerOnlyKeys generates only peer keys for replica nodes
// Replica nodes are untrusted and do not receive the master key or AES key
func CreatePeerOnlyKeys() *CryptoKeys {
	randPeerSeed := make([]byte, 32)
	rand.Read(randPeerSeed)
	peerPriv := ed25519.NewKeyFromSeed(randPeerSeed)
	peerPub := peerPriv.Public().(ed25519.PublicKey)

	return &CryptoKeys{
		MasterPrivateKey: nil, // Will not be set for replica nodes
		MasterPublicKey:  nil, // Will be set during binding
		PeerPrivateKey:   peerPriv,
		PeerPublicKey:    peerPub,
		AESKey:           nil, // Not provided to untrusted replica nodes
	}
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

// Generates Master keypair and AES key from mnemonic and a random peer keypair
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

	//generate encryption key
	AESKey := sha256.Sum256(key)

	return &CryptoKeys{
		MasterPrivateKey: priv,
		MasterPublicKey:  pub,
		PeerPrivateKey:   peerPriv,
		PeerPublicKey:    peerPub,
		AESKey:           AESKey[:],
	}
}

// Hash is computed as: hash(key + value + uint64(size)) for files, hash(key) for folders
func ComputeDataHash(key []byte, value []byte, size int64) []byte {
	h := blake3.New(32, nil)
	sizeBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(sizeBuf, uint64(size))
	h.Write(key)
	if value != nil {
		h.Write(value)
		h.Write(sizeBuf)
	}
	return h.Sum(nil)
}

// Encrypt encrypts data using AES-256-GCM
func Encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-256-GCM
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

const chunkSize = 64 * 1024 // 64KB chunks for streaming encryption

// EncryptStream encrypts a file in chunks using AES-256-GCM and computes hash of encrypted content
// Returns the hash of the encrypted content
func EncryptStream(dst io.Writer, src io.Reader, key []byte, hasher *blake3.Hasher) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	buf := make([]byte, chunkSize)

	for {
		n, err := src.Read(buf)
		if n > 0 {
			nonce := make([]byte, gcm.NonceSize())
			if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
				return err
			}

			ciphertext := gcm.Seal(nonce, nonce, buf[:n], nil)
			if _, err := dst.Write(ciphertext); err != nil {
				return err
			}

			if hasher != nil {
				hasher.Write(ciphertext)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DecryptStream decrypts a file that was encrypted with EncryptStream
func DecryptStream(dst io.Writer, src io.Reader, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonceSize := gcm.NonceSize()
	// Each chunk is: nonce + encrypted data + auth tag
	chunkOverhead := nonceSize + gcm.Overhead()
	encryptedChunkSize := chunkSize + chunkOverhead
	buf := make([]byte, encryptedChunkSize)

	for {
		n, err := io.ReadAtLeast(src, buf, nonceSize+1)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			if n == 0 {
				break
			}
		} else if err != nil {
			return err
		}

		nonce := buf[:nonceSize]
		ciphertext := buf[nonceSize:n]

		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return err
		}

		if _, err := dst.Write(plaintext); err != nil {
			return err
		}

		if n < encryptedChunkSize {
			break
		}
	}
	return nil
}
