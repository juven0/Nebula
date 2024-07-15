package dht

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

// Fonction utilitaire pour créer un hôte libp2p pour les tests
func createTestHost(t *testing.T) (host.Host, error) {
	t.Helper()
	return libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
}

func TestNewNode(t *testing.T) {
	peerID, _ := peer.Decode("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N")
	node := NewNode(peerID, "/ip4/127.0.0.1/tcp/4001", 4001)

	assert.Equal(t, peerID, node.NodeID)
	assert.Equal(t, "/ip4/127.0.0.1/tcp/4001", node.Addr)
	assert.Equal(t, 4001, node.Port)
}

func TestNewDHT(t *testing.T) {
	h, err := createTestHost(t)
	assert.NoError(t, err)
	defer h.Close()

	dht := NewDHT(h)

	assert.NotNil(t, dht)
	assert.NotNil(t, dht.DataStore)
	assert.Equal(t, h, dht.Host)
	assert.Equal(t, h.ID(), dht.RoutingTable.Self.NodeID)
}

func TestStoreAndRetrieve(t *testing.T) {
	h, err := createTestHost(t)
	assert.NoError(t, err)
	defer h.Close()

	dht := NewDHT(h)

	key := "testKey"
	value := []byte("testValue")

	dht.StoreData(key, value)

	retrievedValue, found := dht.Retrieve(key)
	assert.True(t, found)
	assert.Equal(t, value, retrievedValue)
}

func TestFindClosestNodes(t *testing.T) {
	h, err := createTestHost(t)
	assert.NoError(t, err)
	defer h.Close()

	dht := NewDHT(h)

	// Add some nodes to the routing table
	for i := 0; i < 10; i++ {
		peerID, err := peer.Decode(fmt.Sprintf("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5%d", i))
		assert.NoError(t, err)
		node := NewNode(peerID, fmt.Sprintf("/ip4/127.0.0.1/tcp/400%d", i), 4000+i)
		dht.RoutingTable.AddNodeRoutingTable(h, node)
		t.Logf("Added node %d: %s", i, peerID)
	}

	closestNodes := dht.FindClosestNodes("testKey", 5)
	assert.Equal(t, 5, len(closestNodes), "Should find 5 closest nodes")

	for i, node := range closestNodes {
		t.Logf("Closest node %d: %s", i, node.NodeID)
	}
}

func TestXOR(t *testing.T) {
	a, _ := peer.Decode("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N")
	b, _ := peer.Decode("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5M")

	result := XOR(a, b)
	assert.NotEqual(t, big.NewInt(0), result)
}

func TestAddNodeBucket(t *testing.T) {
	h, err := createTestHost(t)
	assert.NoError(t, err)
	defer h.Close()

	bucket := &Bucket{}

	t.Logf("BucketSize constant: %d", BucketSize)

	for i := 0; i < 30; i++ { // Essayons d'ajouter plus de nœuds que BucketSize
		peerID, _ := peer.Decode(fmt.Sprintf("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5%d", i))
		node := NewNode(peerID, fmt.Sprintf("/ip4/127.0.0.1/tcp/400%d", i), 4000+i)
		bucket.AddNodeBucket(h, node)

		t.Logf("After adding node %d, bucket size: %d", i+1, len(bucket.Nodes))
	}

	assert.Equal(t, BucketSize, len(bucket.Nodes), "Bucket size should be equal to BucketSize")

	for i, node := range bucket.Nodes {
		t.Logf("Node %d: %s", i, node.NodeID)
	}
}

func TestAddNodeBucketDirectly(t *testing.T) {
	bucket := &Bucket{}
	h, _ := createTestHost(t)

	for i := 0; i < 30; i++ {
		peerID, _ := peer.Decode(fmt.Sprintf("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5%d", i))
		node := NewNode(peerID, fmt.Sprintf("/ip4/127.0.0.1/tcp/400%d", i), 4000+i)
		bucket.AddNodeBucket(h, node)
		t.Logf("After adding node %d, bucket size: %d", i+1, len(bucket.Nodes))
	}

	assert.Equal(t, BucketSize, len(bucket.Nodes), "Bucket size should be equal to BucketSize")
}
func TestAddNodeRoutingTable(t *testing.T) {
	h, err := createTestHost(t)
	assert.NoError(t, err)
	defer h.Close()

	rt := NewRoutingTable(NewNode(h.ID(), "/ip4/127.0.0.1/tcp/4000", 4000))

	peerID, _ := peer.Decode("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N")
	node := NewNode(peerID, "/ip4/127.0.0.1/tcp/4001", 4001)
	rt.AddNodeRoutingTable(h, node)

	assert.Equal(t, 1, len(rt.Buckets[255].Nodes))
}
