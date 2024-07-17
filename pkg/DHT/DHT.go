package maindht

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"sort"

	//"os/exec"
	//"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ping "github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
)

type messageType int

const (
	IdLength   = 256 / 8
	BucketSize = 10
)

const (
	STORE messageType = iota
	FIND_VALUE
	FIND_NODE
)

type Node struct {
	NodeID peer.ID
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
	Host         host.Host
}

type Message struct {
	Type   messageType
	Key    string
	Value  []byte
	Sender Node
}

func NewNode(peerID peer.ID, addr string, port int) Node {
	return Node{
		NodeID: peerID,
		Addr:   addr,
		Port:   port,
	}
}

func NewDHT(h host.Host) *DHT {
	selfNode := NewNode(h.ID(), h.Addrs()[0].String(), 0)

	return &DHT{
		RoutingTable: *NewRoutingTable(selfNode),
		DataStore:    make(map[string][]byte),
		Host:         h,
	}
}

func (dht *DHT) SendMessage(to peer.ID, message Message) (Message, error) {
	stream, err := dht.Host.NewStream(context.Background(), to, "/dht/1.0.0")
	if err != nil {
		return Message{}, err
	}

	defer stream.Close()

	if err = json.NewEncoder(stream).Encode(message); err != nil {
		return Message{}, fmt.Errorf("could not encode message: %v", err)
	}

	var response Message
	if err := json.NewDecoder(stream).Decode(&response); err != nil {
		return Message{}, fmt.Errorf("could not decode message: %v", err)
	}
	return response, nil
}

func (dht *DHT) HandelIncommingMessages() {
	dht.Host.SetStreamHandler("/dht/1.0.0", func(stream network.Stream) {
		defer stream.Close()

		var msg Message
		if err := json.NewDecoder(stream).Decode(&msg); err != nil {
			log.Printf("Error decoding message: %v", err)
			return
		}

		var response Message
		switch msg.Type {
		case STORE:
			dht.StoreData(msg.Key, msg.Value)
			response = Message{Type: STORE, Key: msg.Key}
		case FIND_VALUE:
			value, found := dht.Retrieve(msg.Key)
			if found {
				response = Message{Type: FIND_VALUE, Key: msg.Key, Value: value}
			} else {
				closestNodes := dht.FindClosestNodes(msg.Key, BucketSize)
				response = Message{Type: FIND_NODE, Key: msg.Key, Value: encodePeers(closestNodes)}
			}
		case FIND_NODE:
			closestNodes := dht.FindClosestNodes(msg.Key, BucketSize)
			response = Message{Type: FIND_NODE, Key: msg.Key, Value: encodePeers(closestNodes)}
		}

		if err := json.NewEncoder(stream).Encode(response); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
	})
}

func (dht *DHT) Retrieve(key string) ([]byte, bool) {
	value, found := dht.DataStore[key]
	if found {
		return value, true
	}

	closestNodes := dht.FindClosestNodes(key, BucketSize)
	for _, node := range closestNodes {
		msg := Message{
			Type:   FIND_VALUE,
			Key:    key,
			Sender: dht.RoutingTable.Self,
		}
		response, err := dht.SendMessage(node.NodeID, msg)
		if err != nil {
			log.Printf("Failed to retrieve data from node %s: %v", node.NodeID, err)
			continue
		}
		if response.Type == FIND_VALUE {
			dht.DataStore[key] = response.Value
			return response.Value, true
		}
	}

	return nil, false
}

func (dht *DHT) FindClosestNodes(key string, count int) []Node {
	keyID, _ := peer.Decode(key)
	var allNodes []Node
	for _, bucket := range dht.RoutingTable.Buckets {
		allNodes = append(allNodes, bucket.Nodes...)
	}

	sort.Slice(allNodes, func(i, j int) bool {
		distI := XOR(keyID, allNodes[i].NodeID)
		distJ := XOR(keyID, allNodes[j].NodeID)
		return distI.Cmp(distJ) < 0
	})

	if len(allNodes) < count {
		return allNodes
	}
	return allNodes[:count]
}
func encodePeers(nodes []Node) []byte {
	var encodedNodes [][]byte
	for _, node := range nodes {
		encodedNode, _ := json.Marshal(node)
		encodedNodes = append(encodedNodes, encodedNode)
	}
	encoded, _ := json.Marshal(encodedNodes)
	return encoded
}

func XOR(a, b peer.ID) *big.Int {
	aBytes, _ := a.MarshalBinary()
	bBytes, _ := b.MarshalBinary()

	aInt := new(big.Int).SetBytes(aBytes)
	bInt := new(big.Int).SetBytes(bBytes)

	return new(big.Int).Xor(aInt, bInt)
}

func (bucket *Bucket) AddNodeBucket(host host.Host, node Node) {
	for _, n := range bucket.Nodes {
		if n.NodeID == node.NodeID {
			return
		}
	}

	if len(bucket.Nodes) < BucketSize {
		bucket.Nodes = append(bucket.Nodes, node)
		return
	}

	bucket.Nodes = append(bucket.Nodes[1:], node)
}

func isActiveNode(h host.Host, node Node) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	peerInfo := peer.AddrInfo{
		ID:    node.NodeID,
		Addrs: []multiaddr.Multiaddr{multiaddr.StringCast(node.Addr)},
	}

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
	if bucketIndex >= len(rt.Buckets) {
		bucketIndex = len(rt.Buckets) - 1
	}
	rt.Buckets[bucketIndex].AddNodeBucket(host, node)
}

func (dht *DHT) FindNode(key string) []Node {
	closestNodes := dht.FindClosestNodes(key, BucketSize)
	queried := make(map[peer.ID]bool)

	for {
		unqueriedNodes := make([]Node, 0)

		for _, node := range closestNodes {
			if !queried[node.NodeID] {
				unqueriedNodes = append(unqueriedNodes, node)
			}
		}

		if len(unqueriedNodes) == 0 {
			break
		}

		for _, node := range unqueriedNodes {
			queried[node.NodeID] = true
			msg := Message{
				Type:   FIND_NODE,
				Key:    key,
				Sender: dht.RoutingTable.Self,
			}

			rsp, err := dht.SendMessage(node.NodeID, msg)
			if err != nil {
				continue
			}
			var newNodes []Node
			json.Unmarshal(rsp.Value, &newNodes)
			for _, newNode := range newNodes {
				dht.RoutingTable.AddNodeRoutingTable(dht.Host, newNode)
				if !queried[newNode.NodeID] {
					closestNodes = append(closestNodes, newNode)
				}
			}
		}

		sort.Slice(closestNodes, func(i, j int) bool {
			distI := XOR(peer.ID(key), closestNodes[i].NodeID)
			distJ := XOR(peer.ID(key), closestNodes[j].NodeID)
			return distI.Cmp(distJ) < 0
		})
		if len(closestNodes) > BucketSize {
			closestNodes = closestNodes[:BucketSize]
		}
	}
	return closestNodes
}

func (dht *DHT) Bootstrap(bootstrapPeer []peer.AddrInfo) error {
	for _, peerInfo := range bootstrapPeer {
		err := dht.Host.Connect(context.Background(), peerInfo)
		if err != nil {
			log.Printf("Failed to connect to bootstrap peer %s: %v", peerInfo.ID, err)
			continue
		}
		dht.RoutingTable.AddNodeRoutingTable(dht.Host, NewNode(peerInfo.ID, peerInfo.Addrs[0].String(), 0))
		dht.FindNode(dht.RoutingTable.Self.NodeID.String())
	}
	return nil
}

func (dht *DHT) StoreData(key string, value []byte) {
	dht.DataStore[key] = value

	closestNodes := dht.FindNode(key)
	for _, node := range closestNodes[:min(len(closestNodes), BucketSize)] {
		msg := Message{
			Type:   STORE,
			Key:    key,
			Value:  value,
			Sender: dht.RoutingTable.Self,
		}
		_, err := dht.SendMessage(node.NodeID, msg)
		if err != nil {
			log.Printf("Failed to store data on node %s: %v", node.NodeID, err)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (dht *DHT) RefreshBuckets() {
	for i := range dht.RoutingTable.Buckets {
		randomID := generateRandomID(i)
		dht.FindNode(randomID)
	}
}

func generateRandomID(prefixLen int) string {
	id := make([]byte, IdLength)
	for i := 0; i < prefixLen; i++ {
		id[i/8] |= 1 << (7 - i%8)
	}
	for i := prefixLen; i < IdLength*8; i++ {
		if rand.Intn(2) == 0 {
			id[i/8] |= 1 << (7 - i%8)
		}
	}
	return peer.ID(id).String()
}
