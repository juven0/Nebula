package main

import (
	//"nebula/pkg/p2p"
	maindht "nebula/pkg/dht"

	"nebula/pkg/p2p"
	"time"
)

// "nebula/pkg/p2p"

func main() {
	network := p2p.P2pNetwork{}
	network.Init()
	dhtConfig := maindht.DHTConfig{
		BucketSize:      32,
		Alpha:           3,
		RefreshInterval: 1 * time.Hour,
	}

	dht := maindht.NewDHT(dhtConfig, network.Host)
	dht.JoinNetwork()
	dht.Start(nil)
	// dhttest.TestSingleNodeDHT()
}
