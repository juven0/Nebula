package dht

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	//"os/exec"
	//"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ping "github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
)

const (
	IdLength   = 256 / 8
	BucketSize = 20
)

type NodeID [IdLength]byte

type Node struct {
	NodeID NodeID
	Addr   string
	Port   int
}

type Bucket struct {
	Nodes []Node
}

type RoutingTable struct {
	Buckets [IdLength * 8]Bucket
	Self    Node
}

type DHT struct {
	RoutingTable RoutingTable
	DataStore    map[string][]byte
}

func NewNode(nodeId NodeID, addr string, port int) Node {
	return Node{
		NodeID: nodeId,
		Addr:   addr,
		Port:   port,
	}
}

func GenerateNodeId(data string) NodeID {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		panic(err)
	}

	// Obtenir le peer.ID à partir de la clé publique
	pid, err := peer.IDFromPublicKey(priv.GetPublic())
	if err != nil {
		panic(err)
	}
	var nodeID NodeID
	copy(nodeID[:], pid)
	return nodeID

}

func XOR(a, b NodeID) big.Int {
	aInt := new(big.Int).SetBytes(a[:])
	bInt := new(big.Int).SetBytes(b[:])

	return *new(big.Int).Xor(aInt, bInt)
}

func (bucket *Bucket) AddNodeBucket(host host.Host, node Node) {
	for _, n := range bucket.Nodes {
		if n.NodeID == node.NodeID {
			return
		}

		if len(bucket.Nodes) < BucketSize {
			bucket.Nodes = append(bucket.Nodes, node)
		} else {
			for i, existingNode := range bucket.Nodes {
				if !isActiveNode(host, existingNode) {
					bucket.Nodes[i] = node
				}

			}
			bucket.Nodes = append(bucket.Nodes[1:], node)
		}
	}
}

func NodeIDToPeerID(nodeID NodeID) (peer.ID, error) {
	hexID := hex.EncodeToString(nodeID[:])
	return peer.Decode(hexID)
}
func isActiveNode(h host.Host, node Node) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	peerId, _ := NodeIDToPeerID(node.NodeID)
	peerInfo := peer.AddrInfo{
		ID:    peerId,
		Addrs: []multiaddr.Multiaddr{multiaddr.StringCast(node.Addr)},
	}

	// Connect to the node
	if err := h.Connect(ctx, peerInfo); err != nil {
		log.Println("Failed to connect to peer:", err)
		return false
	}

	resChan := ping.Ping(ctx, h, peerInfo.ID)
	select {
	case res := <-resChan:
		if res.Error != nil {
			log.Println("Échec du ping:", res.Error)
			return false
		}
		fmt.Println("RTT du ping:", res.RTT)
		return true
	case <-ctx.Done():
		log.Println("Délai d'attente du ping")
		return false
	}
}

func NewRoutingTable(self Node) *RoutingTable {
	rt := &RoutingTable{
		Self: self,
	}
	for i := range rt.Buckets {
		rt.Buckets[i] = Bucket{Nodes: []Node{}}
	}
	return rt
}

func (rt *RoutingTable) AddNodeRoutingTable(host host.Host, node Node) {
	dist := XOR(rt.Self.NodeID, node.NodeID)
	bucketIndex := dist.BitLen() - 1
	rt.Buckets[bucketIndex].AddNodeBucket(host, node)
}

func (dht *DHT) StoreData(key string, value []byte) {
	dht.DataStore[key] = value
	//a completer
}

func (dht *DHT) Retrieve(key string) ([]byte, bool) {
	value, found := dht.DataStore[key]
	//a completer
	return value, found
}
