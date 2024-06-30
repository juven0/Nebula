package blockchains

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type File struct {
	FileId      string
	FileName    string
	FileSize    int64
	FileContent []byte
	Timestamp   string
	Owner       string
}

type FileBlock struct {
	Index     int
	Timestamp string
	Files     []File
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

func CreateFileBlock(preBLock FileBlock, files []File) FileBlock {
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
		Files:     []File{},
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
