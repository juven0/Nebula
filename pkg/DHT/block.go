package maindht

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
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
	newBlock.Hash = newBlock.calculeteHash()
	dht.Blockchain.Blocks = append(dht.Blockchain.Blocks, newBlock)
	return newBlock

}

func (b *Block) calculeteHash() []byte {
	record := fmt.Sprint(b.Index) + b.Timestamp.String() + string(b.Data) + string(b.PrevHash)
	h := sha256.New()
	h.Write([]byte(record))
	return h.Sum(nil)
}
