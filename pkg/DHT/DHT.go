package dht

import (
	"crypto/sha256"
	"math/big"
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

func XOR(a, b NodeID) big.Int {
	aInt := new(big.Int).SetBytes(a[:])
	bInt := new(big.Int).SetBytes(b[:])

	return *new(big.Int).Xor(aInt, bInt)
}

func (bucket *Bucket) AddNodeBucket(node Node) {
	for _, n := range bucket.Nodes {
		if n.NodeID == node.NodeID {
			return
		}

		if len(bucket.Nodes) < BucketSize {
			bucket.Nodes = append(bucket.Nodes, node)
		} else {
			//a faire
			return
		}
	}
}

func (rt *RoutingTable) AddNodeRoutingTable(node Node) {
	dist := XOR(rt.Self.NodeID, node.NodeID)
	bucketIndex := dist.BitLen() - 1
	rt.Buckets[bucketIndex].AddNodeBucket(node)
}
