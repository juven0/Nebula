package dht

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func TestXOR(t *testing.T) {
	peerA, err := peer.Decode("12D3KooWSRZz3VS4v5HmkVq4t6e3VRC5vdsTfTZLhZDfnkWd6FZj")
	assert.NoError(t, err)
	peerB, err := peer.Decode("12D3KooWKXfXjXL5Uzn5FbSiQ84WzgnLBRzA4aRjHTE2cfoU85RA")
	assert.NoError(t, err)

	aBytes, err := peerA.Marshal()
	assert.NoError(t, err)
	bBytes, err := peerB.Marshal()
	assert.NoError(t, err)

	expectedResult := new(big.Int).Xor(
		new(big.Int).SetBytes(aBytes),
		new(big.Int).SetBytes(bBytes),
	)

	actualResult := XOR(peerA, peerB)
	assert.Equal(t, expectedResult, &actualResult, "XOR results do not match")
}

func TestAddNodeBucket(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	assert.NoError(t, err)

	host, err := libp2p.New(
		libp2p.Identity(priv),
	)
	assert.NoError(t, err)

	nodeA := NewNode(host.ID(), "/ip4/127.0.0.1/tcp/12345", 12345)
	nodeB := NewNode(host.ID(), "/ip4/127.0.0.1/tcp/12346", 12346)
	t.Logf("Nodes in bucket after adding nodeA: %+v", nodeB)
	bucket := Bucket{}
	bucket.AddNodeBucket(host, nodeA)
	t.Logf("Nodes in bucket after adding nodeA: %+v", bucket.Nodes)
	assert.Len(t, bucket.Nodes, 1, "Bucket should have 1 node")
	bucket.AddNodeBucket(host, nodeB)
	t.Logf("Nodes in bucket after adding nodeB: %+v", bucket.Nodes)
	assert.Len(t, bucket.Nodes, 2, "Bucket should have 2 nodes")
}

func TestIsActiveNode(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	assert.NoError(t, err)

	host, err := libp2p.New(
		libp2p.Identity(priv),
	)
	assert.NoError(t, err)

	node := NewNode(host.ID(), "/ip4/127.0.0.1/tcp/12345", 12345)

	// Since the node is the same as the host, it should be active
	active := isActiveNode(host, node)
	assert.True(t, active, "Node should be active")
}

func TestAddNodeRoutingTable(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	assert.NoError(t, err)

	host, err := libp2p.New(
		libp2p.Identity(priv),
	)
	assert.NoError(t, err)

	selfNode := NewNode(host.ID(), "/ip4/127.0.0.1/tcp/12345", 12345)
	rt := NewRoutingTable(selfNode)

	nodeB := NewNode(host.ID(), "/ip4/127.0.0.1/tcp/12346", 12346)

	rt.AddNodeRoutingTable(host, nodeB)
	// We should have 1 node in one of the buckets
	nodesCount := 0
	for _, bucket := range rt.Buckets {
		nodesCount += len(bucket.Nodes)
	}
	assert.Equal(t, 1, nodesCount, "Routing table should have 1 node")
}
