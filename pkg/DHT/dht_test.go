package dht

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

// Test de la génération de NodeID
func TestGenerateNodeId(t *testing.T) {
	data := "test_data"
	nodeID := GenerateNodeId(data)
	expectedHash := sha256.Sum256([]byte(data))
	assert.Equal(t, expectedHash[:], nodeID[:], "Les NodeIDs ne correspondent pas")
}

// Test de l'algorithme XOR
func TestXOR(t *testing.T) {
	a := GenerateNodeId("a")
	b := GenerateNodeId("b")
	result := XOR(a, b)
	expectedResult := new(big.Int).Xor(new(big.Int).SetBytes(a[:]), new(big.Int).SetBytes(b[:]))
	assert.Equal(t, expectedResult, &result, "Les résultats XOR ne correspondent pas")
}

// Test de l'ajout de nœud dans le bucket
func TestAddNodeBucket(t *testing.T) {
	nodeID1 := GenerateNodeId("node1")
	nodeID2 := GenerateNodeId("node2")
	node1 := NewNode(nodeID1, "/ip4/127.0.0.1/tcp/4001", 4001)
	node2 := NewNode(nodeID2, "/ip4/127.0.0.1/tcp/4002", 4002)
	bucket := Bucket{Nodes: []Node{}}

	// Ajouter les nœuds
	priv, _, _ := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	h, _ := libp2p.New(libp2p.Identity(priv))

	bucket.AddNodeBucket(h, node1)
	bucket.AddNodeBucket(h, node2)

	assert.Contains(t, bucket.Nodes, node1, "Le nœud 1 devrait être dans le bucket")
	assert.Contains(t, bucket.Nodes, node2, "Le nœud 2 devrait être dans le bucket")
}

// Test de la fonction de ping
func TestPingNode(t *testing.T) {
	// Créer un hôte libp2p
	priv, _, _ := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	h, _ := libp2p.New(libp2p.Identity(priv))

	nodeID := GenerateNodeId("node_ping")
	node := NewNode(nodeID, "/ip4/127.0.0.1/tcp/4001", 4001)

	active := isActiveNode(h, node)
	assert.False(t, active, "Le nœud ne devrait pas être actif")
}

// Test de la conversion de NodeID en PeerID
func TestNodeIDToPeerID(t *testing.T) {
	nodeID := GenerateNodeId("peer_test")
	peerID, err := NodeIDToPeerID(nodeID)
	assert.NoError(t, err, "Il ne devrait pas y avoir d'erreur lors de la conversion")

	// Convertir nodeID en chaîne hexadécimale pour comparaison
	expectedPeerID, _ := peer.Decode(hex.EncodeToString(nodeID[:]))
	assert.Equal(t, expectedPeerID, peerID, "Les PeerIDs ne correspondent pas")
}
