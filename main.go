package main

import (
	//"nebula/pkg/p2p"
	dhttest "nebula/pkg/dht_test"
)

// "nebula/pkg/p2p"

func main() {
	// network := p2p.P2pNetwork{}
	// network.Network()
	dhttest.TestSingleNodeDHT()
}
