package maindht

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (dht *DHT) JoinNetwork() error {

	knowPeers := dht.loadKnowPeers()
	if len(knowPeers)>0 {
		return dht.Bootstrap(knowPeers)
	}

	localPeers, err := dht.discoverLocalPeers()
	if err == nil && len(localPeers) > 0 {
        return dht.Bootstrap(localPeers)
    }

	dnsPeers, err := dht.queryDNSSeeds()
    if err == nil && len(dnsPeers) > 0 {
        return dht.Bootstrap(dnsPeers)
    }

	return fmt.Errorf("failed to join network: no peers found")
}

func (dht *DHT) loadKnowPeers() ([]peer.AddrInfo){

}

func (dht *DHT) discoverLocalPeers()([]peer.AddrInfo, error){

}

func (dht *DHT) queryDNSSeeds() ([]peer.AddrInfo, error) {
    
}

func (dht *DHT) saveKnownPeers(peers []peer.AddrInfo) error {
    
}

