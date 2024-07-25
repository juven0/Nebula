package maindht

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Block struct {
	Index     int64
	Timestamp time.Time
	Data      []byte
	PrevHash  []byte
	Hash      []byte
}

type Blockchain struct {
	Blocks []*Block
	mu     sync.RWMutex
}

func NewGenesisBlock() *Block {
	genesisData := []byte("Genesis Block")
	genesisTime := time.Now()
	genesisBlock := &Block{
		Index:     0,
		Timestamp: genesisTime,
		Data:      genesisData,
		PrevHash:  []byte{},
	}
	genesisBlock.Hash = genesisBlock.calculateHash()
	return genesisBlock
}

func NewBlockchain() *Blockchain {
	return &Blockchain{
		Blocks: []*Block{NewGenesisBlock()},
	}
}

func (dht *DHT) AddBlock(data []byte) *Block {
	dht.Blockchain.mu.Lock()
	defer dht.Blockchain.mu.Unlock()

	prevBlock := dht.Blockchain.Blocks[len(dht.Blockchain.Blocks)-1]
	newBlock := &Block{
		Index:     prevBlock.Index + 1,
		Timestamp: time.Now(),
		Data:      data,
		PrevHash:  prevBlock.Hash,
	}
	newBlock.Hash = newBlock.calculateHash()
	dht.Blockchain.Blocks = append(dht.Blockchain.Blocks, newBlock)
	return newBlock

}

func (b *Block) calculateHash() []byte {
	record := fmt.Sprint(b.Index) + b.Timestamp.String() + string(b.Data) + string(b.PrevHash)
	h := sha256.New()
	h.Write([]byte(record))
	return h.Sum(nil)
}

func (dht *DHT) SyncBlockchain() error {
	closestNodes := dht.FindClosestNodes(dht.RoutingTable.Self.NodeID.String(), BucketSize)

	for _, node := range closestNodes {
		remoteLength, err := dht.getRemoteBlockchainLength(node)
		if err != nil {
			continue
		}

		localLength := len(dht.Blockchain.Blocks)

		if remoteLength > localLength {
			// Demander les blocs manquants
			newBlocks, err := dht.getRemoteBlocks(node, localLength, remoteLength)
			if err != nil {
				continue
			}

			// Vérifier et ajouter les nouveaux blocs
			err = dht.verifyAndAddBlocks(newBlocks)
			if err != nil {
				continue
			}

			log.Printf("Synchronized blockchain with node %s", node.NodeID)
			return nil
		}
	}

	return fmt.Errorf("failed to synchronize blockchain with any node")
}

func (dht *DHT) verifyAndAddBlocks(newBlocks []*Block) error {
	dht.Blockchain.mu.Lock()

	for _, block := range newBlocks {
		if !dht.isValidNewBlock(block, dht.Blockchain.Blocks[len(dht.Blockchain.Blocks)-1]) {
			return fmt.Errorf("invalid block")
		}
		dht.Blockchain.Blocks = append(dht.Blockchain.Blocks, block)
	}
	return nil

}

func (dht *DHT) isValidNewBlock(newBlock, prevBlock *Block) bool {
	if prevBlock.Index+1 != newBlock.Index {
		return false
	}
	if !bytes.Equal(prevBlock.Hash, newBlock.PrevHash) {

		return false
	}
	if !bytes.Equal(newBlock.calculateHash(), newBlock.Hash) {
		return false
	}
	return true
}

func (dht *DHT) getRemoteBlockchainLength(node Node) (int, error) {
	msg := Message{
		Type:   BLOCKCHAIN_LENGTH_REQUEST,
		Sender: dht.RoutingTable.Self,
	}

	response, err := dht.SendMessage(node.NodeID, msg)
	if err != nil {
		return 0, fmt.Errorf("failed to get blockchain length from node %s: %w", node.NodeID, err)
	}

	if response.Type != BLOCKCHAIN_LENGTH_RESPONSE {
		return 0, fmt.Errorf("unexpected response type from node %s", node.NodeID)
	}

	length, err := strconv.Atoi(string(response.Value))
	if err != nil {
		return 0, fmt.Errorf("invalid blockchain length received from node %s: %w", node.NodeID, err)
	}

	return length, nil
}

func (dht *DHT) getRemoteBlocks(node Node, start, end int) ([]*Block, error) {
	msg := Message{
		Type:   BLOCKCHAIN_BLOCKS_REQUEST,
		Sender: dht.RoutingTable.Self,
		Value:  []byte(fmt.Sprintf("%032d%032d", start, end)), // Padding to ensure consistent length
	}

	response, err := dht.SendMessage(node.NodeID, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocks from node %s: %w", node.NodeID, err)
	}

	if response.Type != BLOCKCHAIN_BLOCKS_RESPONSE {
		return nil, fmt.Errorf("unexpected response type from node %s", node.NodeID)
	}

	var blocks []*Block
	err = json.Unmarshal(response.Value, &blocks)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal blocks from node %s: %w", node.NodeID, err)
	}

	if len(blocks) != end-start {
		return nil, fmt.Errorf("received incorrect number of blocks from node %s", node.NodeID)
	}

	return blocks, nil
}

func (dht *DHT) Start(bootstrapPeers []peer.AddrInfo) error {
	dht.mu.Lock()
	defer dht.mu.Unlock()

	if dht.stopCh == nil {
		dht.stopCh = make(chan struct{})
	}

	err := dht.initRoutingTable()
	if err != nil {
		return fmt.Errorf("failed to initialize routing table: %w", err)
	}

	dht.HandelIncommingMessages()
	dht.periodicSync()
	dht.periodicRefresh()
	// dht.periodicMaintenance()

	err = dht.Bootstrap(bootstrapPeers)
	if err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}
	dht.logger.Println("DHT started successfully")
	return nil
}

func (dht *DHT) Stop() error {
	dht.mu.Lock()
	defer dht.mu.Unlock()

	close(dht.stopCh)

	dht.logger.Println("DHT stopped")

	return nil
}

func (dht *DHT) periodicSync() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := dht.SyncBlockchain()
			if err != nil {
				dht.logger.Printf("Failed to sync blockchain: %v", err)
			}
		case <-dht.stopCh:
			return
		}
	}
}

func (dht *DHT) periodicRefresh() {
	ticker := time.NewTicker(dht.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dht.RefreshBuckets()
		case <-dht.stopCh:
			return
		}
	}
}

//a faire

// func (dht *DHT) periodicMaintenance() {
// 	ticker := time.NewTicker(10 * time.Minute)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			dht.performMaintenance()
// 		case <-dht.stopCh:
// 			return
// 		}
// 	}
// }
