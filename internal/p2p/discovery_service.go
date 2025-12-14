package p2p

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/scrypt"
)

const bindProtocolID = "/endershare/bind/1.0"

type ClientInfoMsg struct {
	MasterPublicKeyBase64 string
	PeerID                string
	PeerSignatureBase64   string
}

type ClientInfo struct {
	MasterPublicKey ed25519.PublicKey
	PeerID          peer.ID
	PeerSignature   []byte
	AddrInfo        peer.AddrInfo
}

type challengeResponse struct {
	Result []byte
	Salt   []byte
}

// The client and server mutually verify knowledge of the sync phrase
// Each sends each other a random challenge that must be hashed with the sync phrase

// BindToClient generates a sync phrase and outputs it to the user
// It then advertises the sync phrase and waits for a client to connect
// Once a client connects, it verifies the client knows the sync phrase
// If verification is successful, it reads the client info and returns it
func BindToClient(node *P2PNode) (*ClientInfo, error) {
	syncPhrase := newMnemonic(4)
	ctx, cancelAdvert := context.WithCancel(context.Background())
	defer cancelAdvert()
	node.Advertize(ctx, syncPhrase)
	//create mutex to rate limit this service and prevent brute forcing
	var mutex sync.Mutex
	clientInfo := make(chan *ClientInfo, 1)

	node.host.SetStreamHandler(bindProtocolID, func(s network.Stream) {
		mutex.Lock()
		defer mutex.Unlock()
		defer s.Close()
		time.Sleep(time.Millisecond * 250)

		verifiedPeer, err := mutualVerification(s, syncPhrase)
		if err == nil && verifiedPeer {
			c := &ClientInfoMsg{}
			buf := new(bytes.Buffer)
			_, err = buf.ReadFrom(s)
			if err != nil {
				fmt.Println("Error reading client info:", err)
			}
			err = json.Unmarshal(buf.Bytes(), c)
			if err != nil {
				fmt.Println("Error unmarshaling client info:", err)
				return
			}
			info, err := clientInfoMsgToClientInfo(c)
			if err != nil {
				fmt.Println("Error converting client info message:", err)
				return
			}
			info.AddrInfo = peer.AddrInfo{
				ID:    info.PeerID,
				Addrs: []multiaddr.Multiaddr{s.Conn().RemoteMultiaddr()},
			}
			clientInfo <- info
		}
	})

	//wait for client to connec, time out after 1 hour
	fmt.Println("Waiting for client to bind with sync phrase:", syncPhrase)
	timeout := time.After(time.Hour)
	defer node.host.RemoveStreamHandler(bindProtocolID)
	select {
	case info := <-clientInfo:
		return info, nil
	case <-timeout:
		return nil, fmt.Errorf("timeout waiting for client to bind")
	}
}

// BindNewServer searches for the server and verifies it knows the sync phrase
// Once it finds the new server, it sends the master public key for the server to bind to
// TODO: add context with timeout
func BindNewServer(syncPhrase string, node *P2PNode, masterPubKey ed25519.PublicKey) (*peer.AddrInfo, error) {
	ctx, cancelDiscover := context.WithCancel(context.Background())
	defer cancelDiscover()
	fmt.Printf("Discovering server with phrase: `%s`\n", syncPhrase)
	nodes, err := node.DiscoverPeers(ctx, syncPhrase)
	if err != nil {
		return nil, err
	}

	for peerInfo := range nodes {
		fmt.Println("Found peer", peerInfo.ID)
		err := node.host.Connect(ctx, peerInfo)
		if err != nil {
			fmt.Println("Error connecting to peer:", err)
			continue
		}
		stream, err := node.host.NewStream(ctx, peerInfo.ID, "/endershare/bind/1.0")
		if err != nil {
			fmt.Println("Error creating stream to peer:", err)
			continue
		}
		verifiedPeer, err := mutualVerification(stream, syncPhrase)
		if err != nil {
			fmt.Println("Error during mutual verification:", err)
			continue
		}
		if verifiedPeer {
			fmt.Println("Successfully verified server:", peerInfo.ID)
			//send the master public key to the server
			c := &ClientInfoMsg{
				MasterPublicKeyBase64: hex.EncodeToString(masterPubKey),
			}
			jsonData, err := json.Marshal(c)
			if err != nil {
				fmt.Println("Error marshaling client info:", err)
				continue
			}
			_, err = stream.Write(jsonData)
			if err != nil {
				fmt.Println("Error sending client info to server:", err)
				continue
			}
			return &peerInfo, nil
		}

	}
	return nil, fmt.Errorf("no peers found")
}

func mutualVerification(stream network.Stream, syncPhrase string) (result bool, err error) {
	result = false
	ourChallenge := [32]byte{}
	_, err = rand.Read(ourChallenge[:])
	if err != nil {
		fmt.Println("Error creating challenge:", err)
		return
	}

	//send our challenge
	_, err = stream.Write(ourChallenge[:])
	if err != nil {
		fmt.Println("Error writing challenge to stream:", err)
		return
	}

	//read challenge
	challenge := [32]byte{}
	stream.SetReadDeadline(time.Now().Add(time.Second * 30))
	_, err = stream.Read(challenge[:])
	if err != nil {
		fmt.Println("Error reading from stream:", err)
		return
	}
	ourResponse, err := solveChallenge(syncPhrase, challenge)
	if err != nil {
		fmt.Println("Error solving challenge:", err)
		return
	}
	resp, err := json.Marshal(ourResponse)
	if err != nil {
		return
	}
	stream.Write(resp)

	peerRespBytes := make([]byte, 1024)
	n, err := stream.Read(peerRespBytes)
	if err != nil {
		return
	}
	//unmarshal peer response
	var peerResp challengeResponse
	err = json.Unmarshal(peerRespBytes[:n], &peerResp)
	if err != nil {
		fmt.Println("Error unmarshalling peer response")
		return
	}

	return verifyChallengeResponse(syncPhrase, ourChallenge, peerResp), nil
}

func solveChallenge(syncPhrase string, challenge [32]byte) (challengeResponse, error) {
	salt := [32]byte{}
	_, err := rand.Read(salt[:])
	if err != nil {
		return challengeResponse{}, err
	}
	key, err := scrypt.Key(append([]byte(syncPhrase), challenge[:]...), salt[:], 32768, 8, 1, 32)
	return challengeResponse{
		Result: key,
		Salt:   salt[:],
	}, err
}

func verifyChallengeResponse(syncPhrase string, challenge [32]byte, response challengeResponse) bool {
	key, err := scrypt.Key(append([]byte(syncPhrase), challenge[:]...), response.Salt, 32768, 8, 1, 32)
	if err != nil {
		return false
	}
	return bytes.Equal(key, response.Result)
}

func clientInfoMsgToClientInfo(msg *ClientInfoMsg) (*ClientInfo, error) {
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
	for i := range numWords {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(wordList))))
		words[i] = wordList[n.Int64()]
	}
	return strings.Join(words, " ")
}
