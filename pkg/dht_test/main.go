package dhttest

import (
	"bytes"
	"log"
	"time"

	maindht "nebula/pkg/dht"

	"github.com/libp2p/go-libp2p"
)

func TestSingleNodeDHT() {

	h, err := libp2p.New()
	if err != nil {
		log.Fatal(err)
	}

	dhtConfig := maindht.DHTConfig{
		BucketSize:      32,
		Alpha:           3,
		RefreshInterval: 1 * time.Hour,
	}

	dht := maindht.NewDHT(dhtConfig, h)
	err = dht.Start(nil)
	if err != nil {
		log.Fatal(err)
	}

	key := "testkey"
	value := []byte("testvalue")

	dht.StoreData(key, value)
	retrievedVallue, found := dht.Retrieve(key)
	if !found || !bytes.Equal(value, retrievedVallue) {
		log.Fatal("La récupération des données a échoué")
	}

	file := maindht.File{
		Name:    "testFile.txt",
		Size:    1024,
		Hash:    "testHash",
		OwnerID: h.ID(),
	}

	fileData := []byte("Contenu du fichier test")
	err = dht.StoreFile(file, fileData)
	if err != nil {
		log.Fatal("Le stockage du fichier a échoué")
	}

	blockData := []byte("Données du bloc test")
	newBlock := dht.AddBlock(blockData)
	if newBlock == nil {
		log.Fatal("L'ajout du bloc a échoué")
	}

	dht.Stop()
}
