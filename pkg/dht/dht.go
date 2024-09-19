package maindht

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	//"os/exec"
	//"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"

	// ping "github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multistream"
)

type messageType int

// const proto = protocol.ID("/ipfs/ping/1.0.0")
const (
	pingProtocol    = "/ping/1.0.0"
	messageProtocol = "/message/1.0.0"
)
const (
	IdLength   = 256 / 8
	BucketSize = 10
)

const messageDelimiter = byte('\x00')

const (
	STORE messageType = iota
	FIND_VALUE
	FIND_NODE
	DELETE_FILE
	BLOCKCHAIN_BLOCKS_RESPONSE
	BLOCKCHAIN_BLOCKS_REQUEST
	BLOCKCHAIN_LENGTH_REQUEST
	BLOCKCHAIN_LENGTH_RESPONSE
	CONNECTION_SUCCESSFUL
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
	RoutingTable *RoutingTable
	DataStore    map[string][]byte
	Host         host.Host
	Blockchain   *Blockchain
	mu           sync.RWMutex
	stopCh       chan struct{}
	config       DHTConfig
	metrics      DHTMetrics
	logger       *log.Logger
}

type DHTConfig struct {
	BucketSize        int
	Alpha             int // Nombre de requêtes parallèles pour les opérations de lookup
	RefreshInterval   time.Duration
	ReplicationFactor int
}

type DHTMetrics struct {
	TotalRequests     int64
	SuccessfulLookups int64
	FailedLookups     int64
	StorageUsage      int64
}

type Message struct {
	Type   messageType
	Key    string
	Value  []byte
	Sender Node
}

type File struct {
	Name       string
	Size       int64
	Hash       string
	OwnerID    peer.ID
	Permission int
	Timestamp  time.Time
}

func NewNode(peerID peer.ID, addr string, port int) Node {

	return Node{
		NodeID: peerID,
		Addr:   addr,
		Port:   port,
	}
}

func (dht *DHT) handleStream(s network.Stream) {
	defer s.Close()

	// Créer un multiplexeur multistream
	mux := multistream.NewMultistreamMuxer[string]()
	mux.AddHandler(messageProtocol, dht.handleMessage)

	err := mux.Handle(s)
	if err != nil {
		if err == io.EOF {
			dht.logger.Printf("Stream closed by remote peer")
		} else if strings.Contains(err.Error(), "message did not have trailing newline") {
			dht.logger.Printf("Received message without trailing newline, trying to handle anyway")
			// Essayez de lire le message sans le retour à la ligne
			buf, _ := io.ReadAll(s)
			dht.logger.Printf("Received raw message: %s", string(buf))
			// Vous pouvez essayer de traiter le message ici si nécessaire
		} else {
			dht.logger.Printf("Failed to handle stream: %v", err)
		}
	}
}

func NewDHT(cfg DHTConfig, h host.Host) *DHT {
	// Créer un Node à partir de l'host
	selfNode := NewNode(h.ID(), h.Addrs()[0].String(), 0) // Nous utilisons 0 comme port par défaut ici

	dht := &DHT{
		DataStore:    make(map[string][]byte),
		RoutingTable: NewRoutingTable(selfNode, cfg.BucketSize),
		Host:         h,
		Blockchain:   NewBlockchain(),
		stopCh:       make(chan struct{}),
		logger:       log.New(os.Stdout, "DHT: ", log.Ldate|log.Ltime|log.Lshortfile),
	}

	mux := multistream.NewMultistreamMuxer[string]()
	mux.AddHandler(messageProtocol, dht.handleMessage)

	// Définir le gestionnaire de flux pour l'hôte
	h.SetStreamHandler(protocol.ID(messageProtocol), dht.handleStream)
	return dht
}

func (dht *DHT) handleMessage(proto string, rwc io.ReadWriteCloser) error {
	defer rwc.Close()

	reader := bufio.NewReader(rwc)

	for {
		// Read message length (4 bytes)
		lengthBuf := make([]byte, 4)
		_, err := io.ReadFull(reader, lengthBuf)
		if err != nil {
			if err == io.EOF {
				return nil // Connection closed normally
			}
			return fmt.Errorf("failed to read message length: %w", err)
		}

		messageLength := binary.BigEndian.Uint32(lengthBuf)

		// Read the message
		messageBuf := make([]byte, messageLength)
		_, err = io.ReadFull(reader, messageBuf)
		if err != nil {
			return fmt.Errorf("failed to read message: %w", err)
		}

		var msg Message
		if err := json.Unmarshal(messageBuf, &msg); err != nil {
			dht.logger.Printf("Error decoding message: %v, raw message: %x", err, messageBuf)
			return err
		}

		dht.logger.Printf("Received message from peer. Type: %v, Key: %s, Value: %s", msg.Type, msg.Key, string(msg.Value))

		response := dht.processMessage(msg)

		// Encode response
		responseBytes, err := json.Marshal(response)
		if err != nil {
			dht.logger.Printf("Error encoding response: %v", err)
			return err
		}

		// Write response length
		binary.BigEndian.PutUint32(lengthBuf, uint32(len(responseBytes)))
		if _, err := rwc.Write(lengthBuf); err != nil {
			return fmt.Errorf("failed to write response length: %w", err)
		}

		// Write response
		if _, err := rwc.Write(responseBytes); err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}
}

// func (dht *DHT) handleMessage(proto string, rwc io.ReadWriteCloser) error {
// 	defer rwc.Close()

// 	decoder := json.NewDecoder(rwc)
// 	encoder := json.NewEncoder(rwc)

// 	for {
// 		var msg Message
// 		if err := decoder.Decode(&msg); err != nil {
// 			if err == io.EOF {
// 				return nil // Connection closed normally
// 			}
// 			dht.logger.Printf("Error decoding message: %v", err)
// 			return err
// 		}

// 		dht.logger.Printf("Received message from peer. Type: %v, Key: %s, Value: %s", msg.Type, msg.Key, string(msg.Value))

// 		response := dht.processMessage(msg)

// 		if err := encoder.Encode(response); err != nil {
// 			dht.logger.Printf("Error encoding response: %v", err)
// 			return err
// 		}
// 	}
// }

// func (dht *DHT) handleMessage(proto string, rwc io.ReadWriteCloser) error {
// 	defer rwc.Close()

// 	scanner := bufio.NewScanner(rwc)
// 	for scanner.Scan() {
// 		line := scanner.Bytes()

// 		var msg Message
// 		if err := json.Unmarshal(line, &msg); err != nil {
// 			dht.logger.Printf("Error decoding message: %v, raw message: %s", err, string(line))
// 			return err
// 		}

// 		dht.logger.Printf("Received message from peer. Type: %v, Key: %s, Value: %s", msg.Type, msg.Key, string(msg.Value))

// 		response := dht.processMessage(msg)

// 		responseJSON, err := json.Marshal(response)
// 		if err != nil {
// 			dht.logger.Printf("Error encoding response: %v", err)
// 			return err
// 		}

// 		if _, err := rwc.Write(append(responseJSON, '\n')); err != nil {
// 			dht.logger.Printf("Error writing response: %v", err)
// 			return err
// 		}
// 	}

// 	if err := scanner.Err(); err != nil {
// 		dht.logger.Printf("Error reading from stream: %v", err)
// 		return err
// 	}

//		return nil
//	}
func (dht *DHT) processMessage(msg Message) Message {
	var response Message

	switch msg.Type {
	case CONNECTION_SUCCESSFUL:
		dht.logger.Printf("Connection successful message received: %s", string(msg.Value))
		response = Message{Type: CONNECTION_SUCCESSFUL, Value: []byte("Connection acknowledged")}
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
	case DELETE_FILE:
		delete(dht.DataStore, msg.Key)
		response = Message{Type: DELETE_FILE, Key: msg.Key}
	default:
		dht.logger.Printf("Unknown message type received: %v", msg.Type)
		response = Message{Type: messageType(-1), Value: []byte("Unknown message type")}
	}

	return response
}

// func NewDHT(h host.Host) *DHT {
// 	selfNode := NewNode(h.ID(), h.Addrs()[0].String(), 0)

// 	return &DHT{
// 		RoutingTable: *NewRoutingTable(selfNode),
// 		DataStore:    make(map[string][]byte),
// 		Host:         h,
// 	}
// }

func (dht *DHT) SendMessage(to peer.ID, message Message) (Message, error) {
	ctx := context.Background()
	s, err := dht.Host.NewStream(ctx, to, protocol.ID(messageProtocol))
	if err != nil {
		return Message{}, fmt.Errorf("failed to open stream: %w", err)
	}
	defer s.Close()

	// Encode the message
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return Message{}, fmt.Errorf("failed to encode message: %w", err)
	}

	// Write message length
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(messageBytes)))
	if _, err := s.Write(lengthBuf); err != nil {
		return Message{}, fmt.Errorf("failed to write message length: %w", err)
	}

	// Write message
	if _, err := s.Write(messageBytes); err != nil {
		return Message{}, fmt.Errorf("failed to write message: %w", err)
	}

	// Read response length
	_, err = io.ReadFull(s, lengthBuf)
	if err != nil {
		return Message{}, fmt.Errorf("failed to read response length: %w", err)
	}

	responseLength := binary.BigEndian.Uint32(lengthBuf)

	// Read response
	responseBuf := make([]byte, responseLength)
	_, err = io.ReadFull(s, responseBuf)
	if err != nil {
		return Message{}, fmt.Errorf("failed to read response: %w", err)
	}

	var response Message
	if err := json.Unmarshal(responseBuf, &response); err != nil {
		return Message{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}

// func (dht *DHT) SendMessage(to peer.ID, message Message) (Message, error) {
// 	ctx := context.Background()
// 	s, err := dht.Host.NewStream(ctx, to, protocol.ID(messageProtocol))
// 	if err != nil {
// 		return Message{}, fmt.Errorf("failed to open stream: %w", err)
// 	}
// 	defer s.Close()

// 	// Encode the message
// 	jsonMessage, err := json.Marshal(message)
// 	if err != nil {
// 		return Message{}, fmt.Errorf("failed to encode message: %w", err)
// 	}

// 	// Add length prefix
// 	lenBuf := make([]byte, 4)
// 	binary.BigEndian.PutUint32(lenBuf, uint32(len(jsonMessage)))

// 	// Send length prefix and message
// 	if _, err = s.Write(lenBuf); err != nil {
// 		return Message{}, fmt.Errorf("failed to write message length: %w", err)
// 	}
// 	if _, err = s.Write(jsonMessage); err != nil {
// 		return Message{}, fmt.Errorf("failed to write message: %w", err)
// 	}

// 	// Read response length
// 	if _, err = io.ReadFull(s, lenBuf); err != nil {
// 		return Message{}, fmt.Errorf("failed to read response length: %w", err)
// 	}
// 	responseLen := binary.BigEndian.Uint32(lenBuf)

// 	// Read response
// 	responseBytes := make([]byte, responseLen)
// 	if _, err = io.ReadFull(s, responseBytes); err != nil {
// 		return Message{}, fmt.Errorf("failed to read response: %w", err)
// 	}

// 	var response Message
// 	if err = json.Unmarshal(responseBytes, &response); err != nil {
// 		return Message{}, fmt.Errorf("failed to decode response: %w", err)
// 	}

// 	return response, nil
// }

// func (dht *DHT) SendMessage(to peer.ID, message Message) (Message, error) {
// 	ctx := context.Background()
// 	s, err := dht.Host.NewStream(ctx, to, protocol.ID(messageProtocol))
// 	if err != nil {
// 		return Message{}, fmt.Errorf("failed to open stream: %w", err)
// 	}
// 	defer s.Close()

// 	encoder := json.NewEncoder(s)
// 	if err = encoder.Encode(message); err != nil {
// 		return Message{}, fmt.Errorf("failed to encode message: %w", err)
// 	}

// 	// Ajouter un retour à la ligne après le message JSON
// 	if _, err := s.Write([]byte("\n")); err != nil {
// 		return Message{}, fmt.Errorf("failed to write newline: %w", err)
// 	}

// 	var response Message
// 	decoder := json.NewDecoder(s)
// 	if err = decoder.Decode(&response); err != nil {
// 		return Message{}, fmt.Errorf("failed to decode response: %w", err)
// 	}

// 	return response, nil
// }

func (dht *DHT) HandelIncommingMessages() {
	dht.Host.SetStreamHandler(messageProtocol, func(stream network.Stream) {
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
		case DELETE_FILE:
			delete(dht.DataStore, msg.Key)
			response = Message{Type: DELETE_FILE, Key: msg.Key}

		case BLOCKCHAIN_LENGTH_REQUEST:
			length := len(dht.Blockchain.Blocks)
			response = Message{Type: BLOCKCHAIN_LENGTH_RESPONSE, Value: []byte(strconv.Itoa(length))}
		case BLOCKCHAIN_BLOCKS_REQUEST:
			start, _ := strconv.Atoi(string(msg.Value[:32]))
			end, _ := strconv.Atoi(string(msg.Value[32:]))
			blocks := dht.Blockchain.Blocks[start:end]
			blocksData, _ := json.Marshal(blocks)
			response = Message{Type: BLOCKCHAIN_BLOCKS_RESPONSE, Value: blocksData}
		case CONNECTION_SUCCESSFUL:
			log.Printf("Received connection message from peer %s: %s", stream.Conn().RemotePeer().String(), string(msg.Value))
			return

		}

		if err := json.NewEncoder(stream).Encode(response); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
	})
}

func (dht *DHT) Retrieve(key string) ([]byte, bool) {
	dht.mu.RLock()
	value, found := dht.DataStore[key]
	dht.mu.RUnlock()

	if found {
		return value, true
	}

	closestNodes := dht.FindClosestNodes(key, dht.config.Alpha)
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

func NewRoutingTable(self Node, bucketSize int) *RoutingTable {
	rt := &RoutingTable{
		Self: self,
	}
	for i := range rt.Buckets {
		rt.Buckets[i] = Bucket{Nodes: make([]Node, 0, bucketSize)}
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

func (dht *DHT) initRoutingTable() error {
	return nil
}

func (dht *DHT) Bootstrap(bootstrapPeer []peer.AddrInfo) error {
	var wg sync.WaitGroup

	successfulConnections := 0
	var mu sync.RWMutex

	for _, peerInfo := range bootstrapPeer {
		wg.Add(1)
		go func(peer peer.AddrInfo) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			err := dht.Host.Connect(ctx, peer)
			if err != nil {
				log.Printf("Failed to connect to bootstrap peer %s: %v", peer.ID, err)
				return
			}

			if !dht.checkProtocolSupport(peer.ID) {
				dht.logger.Printf("Peer %s does not support protocol %s. Trying fallback.", peer.ID, messageProtocol)
				// Essayez un protocole de repli ou une autre méthode de communication
				if err := dht.tryFallbackProtocol(peer.ID); err != nil {
					dht.logger.Printf("Fallback failed for peer %s: %v", peer.ID, err)
					return
				}
			}
			//dht.checkProtocolSupport(peer.ID)
			mu.Lock()
			successfulConnections++
			mu.Unlock()

			if err := dht.sendConnectionMessage(peer.ID); err != nil {
				dht.logger.Printf("Failed to send connection message to peer %s: %v", peer.ID, err)
			}

			dht.RoutingTable.AddNodeRoutingTable(dht.Host, NewNode(peer.ID, peer.Addrs[0].String(), 0))

			closestNodes := dht.FindNode(peer.ID.String())
			for _, node := range closestNodes {
				dht.RoutingTable.AddNodeRoutingTable(dht.Host, node)
			}

			ownClosestNodes := dht.FindNode(dht.RoutingTable.Self.NodeID.String())
			for _, node := range ownClosestNodes {
				dht.RoutingTable.AddNodeRoutingTable(dht.Host, node)
			}
		}(peerInfo)
	}
	wg.Wait()

	if successfulConnections == 0 {
		return fmt.Errorf("failed to connect to any bootstrap peers")
	}

	log.Printf("Successfully bootstrapped with %d peers", successfulConnections)
	return nil
}

func (dht *DHT) tryFallbackProtocol(peerID peer.ID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Utilisez directement la fonction Ping du package
	result := <-ping.Ping(ctx, dht.Host, peerID)
	if result.Error != nil {
		return fmt.Errorf("ping failed: %w", result.Error)
	}

	dht.logger.Printf("Successful ping to %s (RTT: %s)", peerID, result.RTT)
	return nil
}

func (dht *DHT) checkProtocolSupport(peerID peer.ID) bool {
	supported, err := dht.Host.Peerstore().SupportsProtocols(peerID, messageProtocol)
	if err != nil {
		log.Printf("Error checking protocol support for peer %s: %v", peerID, err)
		return false
	}
	return len(supported) > 0
}

func (dht *DHT) sendConnectionMessage(peerID peer.ID) error {
	message := Message{
		Type:  CONNECTION_SUCCESSFUL,
		Value: []byte(fmt.Sprintf("Hello from %s", dht.Host.ID().String())),
	}
	dht.logger.Printf("Sending connection message to peer %s: %+v", peerID, message)
	jsonMessage, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = dht.SendMessage(peerID, Message{Type: CONNECTION_SUCCESSFUL, Value: jsonMessage})
	return err
}

func (dht *DHT) StoreData(key string, value []byte) error {
	dht.mu.Lock()
	defer dht.mu.Unlock()

	if _, exists := dht.DataStore[key]; exists {
		return fmt.Errorf("key already exists: %s", key)
	}

	dht.DataStore[key] = value

	closestNodes := dht.FindClosestNodes(key, dht.config.ReplicationFactor)
	// closestNodes := dht.FindNode(key)

	var wg sync.WaitGroup
	errChan := make(chan error, len(closestNodes))

	for _, node := range closestNodes[:min(len(closestNodes), BucketSize)] {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			msg := Message{
				Type:   STORE,
				Key:    key,
				Value:  value,
				Sender: dht.RoutingTable.Self,
			}
			_, err := dht.SendMessage(node.NodeID, msg)
			if err != nil {
				errChan <- fmt.Errorf("failed to replicate on node %s: %v", n.NodeID, err)
			}
		}(node)

	}
	wg.Wait()
	close(errChan)

	var replicationErrors []error
	for err := range errChan {
		replicationErrors = append(replicationErrors, err)
	}

	if len(replicationErrors) > 0 {
		return fmt.Errorf("replication errors occurred: %v", replicationErrors)
	}

	return nil
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

func (dht *DHT) StoreFile(file File, data []byte) error {
	fileJson, err := json.Marshal(file)
	if err != nil {
		return err
	}

	key := file.Hash
	value := append(fileJson, data...)
	dht.StoreData(key, value)

	return nil
}

func (dht *DHT) RetrieveFile(hash string) (*File, []byte, error) {
	value, found := dht.Retrieve(hash)
	if !found {
		return nil, nil, fmt.Errorf("file not found")
	}

	var file File
	err := json.Unmarshal(value[:256], &file)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal file metadata: %v", err)
	}

	data := value[:256]
	return &file, data, nil
}

func (dht *DHT) UpdateFile(File File, data []byte, UpdaterID peer.ID) error {
	existingFile, _, err := dht.RetrieveFile(File.Hash)
	if err != nil {
		return err
	}

	if existingFile.OwnerID != UpdaterID {
		return fmt.Errorf("permission denied: only the owner can update the file")
	}
	return dht.StoreFile(File, data)
}

func (dht *DHT) DeleteFile(hash string, deleterID peer.ID) error {
	file, _, err := dht.RetrieveFile(hash)
	if err != nil {
		return err
	}

	if file.OwnerID != deleterID {
		return fmt.Errorf("permission denied: only the owner can update the file")
	}
	delete(dht.DataStore, hash)

	deleteMsg := Message{
		Type:   DELETE_FILE,
		Key:    hash,
		Sender: dht.RoutingTable.Self,
	}
	closestNodes := dht.FindClosestNodes(hash, BucketSize)

	for _, node := range closestNodes {
		go func(n Node) {
			_, err := dht.SendMessage(n.NodeID, deleteMsg)
			if err != nil {
				log.Printf("Failed to send delete message to node %s: %v", n.NodeID, err)
			}
		}(node)
	}
	return nil
}
func (dht *DHT) ListUserFiles(ownerID peer.ID) ([]File, error) {
	var userFiles []File

	for _, value := range dht.DataStore {
		var file File
		err := json.Unmarshal(value[:256], &file)
		if err == nil && file.OwnerID == ownerID {
			userFiles = append(userFiles, file)
		}
	}

	return userFiles, nil
}
