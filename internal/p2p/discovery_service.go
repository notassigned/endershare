package p2p

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tyler-smith/go-bip39"
)

type ClientInfoMsg struct {
	MasterPublicKeyBase64 string
	PeerID                string
	PeerSignatureBase64   string
}

type ClientInfo struct {
	MasterPublicKey ed25519.PublicKey
	PeerID          peer.ID
	PeerSignature   []byte
}

func BindNewClient(node P2PNode) (*ClientInfo, error) {
	syncPhrase := newMnemonic(4)
	//create mutex to rate limit this service and prevent brute forcing
	var mutex sync.Mutex
	clientInfo := make(chan *ClientInfo, 1)

	node.libp2pNode.SetStreamHandler("/endershare/bind/1.0", func(s network.Stream) {
		mutex.Lock()
		defer mutex.Unlock()
		defer s.Close()
		time.Sleep(time.Millisecond * 250)

		//create challenge
		randomBytes := make([]byte, 32)
		_, err := rand.Read(randomBytes)
		if err != nil {
			return
		}
		challenge := hex.EncodeToString(randomBytes)
		expectedProof := sha256.Sum256([]byte(syncPhrase + challenge))

		//send challenge
		s.Write([]byte(challenge))

		//expect back a hex sha256 of (syncPhrase + challenge)
		theirResponse := make([]byte, 64)
		_, err = s.Read(theirResponse)
		if err != nil {
			fmt.Println("Error reading from stream:", err)
			return
		}
		theirProof, err := hex.DecodeString(string(theirResponse))
		if err != nil {
			return
		}

		if !bytes.Equal(expectedProof[:], theirProof) {
			fmt.Println("Client failed verification")
			return
		}

		fmt.Println("Client verified")
		//allocate a buffer and read stream until closed
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(s)
		if err != nil {
			fmt.Println("Error reading client info:", err)
			return
		}

		//parse client info
		var clientInfoMsg ClientInfoMsg
		err = json.Unmarshal(buf.Bytes(), &clientInfo)
		if err != nil {
			fmt.Println("Error unmarshaling client info:", err)
			return
		}
		c, err := ClientInfoMsgToClientInfo(clientInfoMsg)
		if err == nil {
			clientInfo <- c
		}
	})

	//wait for client to connec, time out after 1 hour
	fmt.Println("Waiting for client to bind with sync phrase:", syncPhrase)
	timeout := time.After(time.Hour)
	select {
	case info := <-clientInfo:
		return info, nil
	case <-timeout:
		return nil, fmt.Errorf("timeout waiting for client to bind")
	}
}

// BindNewServer mutually verifies that the client and server both know the sync phrase
// We verify the server knows the phrase by checking that challenge = sha256(syncPhrase + peerID)
func BindNewServer(syncPhrase string, node P2PNode) (*peer.AddrInfo, error) {
	ctx := context.Background()
	nodes, err := node.EnableRoutingDiscovery(ctx, syncPhrase)

	if err != nil {
		return nil, err
	}
	for peerInfo := range nodes {
		fmt.Println("Found peer", peerInfo.ID)
		err := node.libp2pNode.Connect(ctx, peerInfo)
		if err != nil {
			fmt.Println("Error connecting to peer:", err)
			continue
		}
		stream, err := node.libp2pNode.NewStream(ctx, peerInfo.ID, "/endershare/bind/1.0")
		if err != nil {
			fmt.Println("Error creating stream to peer:", err)
			continue
		}

		//read challenge
		challenge := make([]byte, 64)
		_, err = stream.Read(challenge)
		if err != nil {
			fmt.Println("Error reading from stream:", err)
			continue
		}
		ourProof := sha256.Sum256([]byte(syncPhrase + string(challenge)))
		stream.Write(ourProof[:])
		//todo: verify server

		return &peerInfo, nil
	}
	return nil, fmt.Errorf("no peers found")
}

func ClientInfoMsgToClientInfo(msg ClientInfoMsg) (*ClientInfo, error) {
	masterPubKeyBytes, err := hex.DecodeString(msg.MasterPublicKeyBase64)
	if err != nil {
		return nil, err
	}
	peerID, err := peer.Decode(msg.PeerID)
	if err != nil {
		return nil, err
	}
	peerSignature, err := hex.DecodeString(msg.PeerSignatureBase64)
	if err != nil {
		return nil, err
	}
	return &ClientInfo{
		MasterPublicKey: ed25519.PublicKey(masterPubKeyBytes),
		PeerID:          peerID,
		PeerSignature:   peerSignature,
	}, nil
}

func newMnemonic(numWords int) string {
	wordList := bip39.GetWordList()
	words := make([]string, numWords)
	for i := 0; i < numWords; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(wordList))))
		words[i] = wordList[n.Int64()]
	}
	return strings.Join(words, " ")
}
