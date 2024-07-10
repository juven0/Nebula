package dht

import (
	"crypto/sha256"
)

const (
	IdLength   = 256 / 8
	BucketSize = 20
)

type NodeID [IdLength]byte

type Node struct {
	NodeID NodeID
	Addr   string
}

type Bucket struct {
	Nodes []Node
}

type RoutingTable struct {
	Buckets [IdLength * 8]Bucket
	Self    Node
}

func GenerateNodeId(data string) NodeID {
	hash := sha256.Sum256([]byte(data))
	var ID NodeID
	copy(ID[:], hash[:])

	return ID
}
