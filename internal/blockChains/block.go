package blockchains

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	file "nebula/internal/files"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type FileBlock struct {
	Index     int
	Timestamp string
	Files     []file.File
	PrevHash  string
	Hash      string
	Nonce     int
}

type FileBLockChain struct {
	Blocks []FileBlock
}

func (fb *FileBlock) CalculeteFileHash() string {
	record := string(rune(fb.Index)) + fb.Timestamp + string(rune(fb.Nonce)) + fb.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func CreateFileBlock(preBLock FileBlock, files []file.File) FileBlock {
	block := FileBlock{
		Index:     preBLock.Index + 1,
		Timestamp: time.Now().String(),
		Files:     files,
		PrevHash:  preBLock.PrevHash,
		Nonce:     0,
	}
	block.Hash = block.CalculeteFileHash()
	return block
}

func CreateFileGenesisBlock() FileBlock {
	return FileBlock{
		Index:     0,
		Timestamp: time.Now().String(),
		Files:     []file.File{},
		PrevHash:  "",
		Hash:      "",
		Nonce:     0,
	}
}

func InitializeFileBlockchain() []FileBlock {
	genesisBlock := CreateFileGenesisBlock()
	return []FileBlock{genesisBlock}
}

func AddFileBlock(bc *[]FileBlock, newBlock FileBlock) {
	if isValidFileBlock(newBlock, (*bc)[len(*bc)-1]) {
		*bc = append(*bc, newBlock)
	}
}

func isValidFileBlock(newBlock, prevBlock FileBlock) bool {
	if prevBlock.Index+1 != newBlock.Index {
		return false
	}
	if prevBlock.Hash != newBlock.PrevHash {
		return false
	}
	if newBlock.Hash != newBlock.CalculeteFileHash() {
		return false
	}
	return true
}

func HandleFileStream(s network.Stream, fileBlockchain *[]FileBlock) {
	var newFileBlock FileBlock
	decoder := json.NewDecoder(s)
	err := decoder.Decode(&newFileBlock)
	if err != nil {
		log.Println("Failed to decode new file block:", err)
		return
	}
	AddFileBlock(fileBlockchain, newFileBlock)
	fmt.Println("New file block added to the blockchain:", newFileBlock)
	DisplayTransactions(*fileBlockchain)
	s.Close()
}

func sendFileBLock(h host.Host, target peer.AddrInfo, fileBlock FileBlock) {
	s, err := h.NewStream(context.Background(), target.ID, "/p2p/1.0.0")
	if err != nil {
		log.Println("Erreur lors de la création du flux:", err)
		return
	}
	encoder := json.NewEncoder(s)
	err = encoder.Encode(fileBlock)
}

// check all block

func DisplayTransactions(blockchain []FileBlock) {
	fmt.Println("Current Blockchain State:")
	for _, block := range blockchain {
		fmt.Printf("Block Index: %d\n", block.Index)
		fmt.Printf("Timestamp: %s\n", block.Timestamp)
		fmt.Printf("Previous Hash: %s\n", block.PrevHash)
		fmt.Printf("Hash: %s\n", block.Hash)
		fmt.Println("Files:")
		for _, file := range block.Files {
			fmt.Printf("  File ID: %s\n", file.FileId)
			fmt.Printf("  File Name: %s\n", file.FileName)
			fmt.Printf("  File Size: %d\n", file.FileSize)
			fmt.Printf("  Owner: %s\n", file.Owner)
		}
		fmt.Println()
	}
}
