package files

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

const BLOCK_SIZE = 1024 * 1024

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

func (fb *FileBlock) CalculeteFileHash() string {
	record := string(rune(fb.Index)) + fb.Timestamp + string(rune(fb.Nonce)) + fb.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func SpliteFile(filePath string) ([]string, error) {
	var hashes []string
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buff := make([]byte, BLOCK_SIZE)

	for {
		byteRead, err := file.Read(buff)
		if err != nil && err != io.EOF {
			return nil, err
		}

		if byteRead == 0 {
			break
		}

		filePart := buff[:byteRead]
		blockHash := generateHash(filePart)
		hashes = append(hashes, blockHash)

		blockFileName := fmt.Sprintf("%s_%s.blk", filePath, blockHash)
		err = os.WriteFile(blockFileName, filePart, 0644)
		if err != nil {
			return nil, err
		}
	}

	return hashes, nil
}
