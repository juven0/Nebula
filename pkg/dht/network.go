package maindht

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
)

type mdnsNotifee struct {
    h host.Host
}

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
    if err := n.h.Connect(context.Background(), pi); err != nil {
        log.Printf("Error connecting to peer: %s", err)
    } else {
        log.Printf("Connected to peer: %s", pi.ID)
    }
	
}


func (dht *DHT) JoinNetwork() error {

	knowPeers, err := dht.loadKnownPeers()
	if err != nil {
		return err
	}
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

func (dht *DHT) loadKnownPeers() ([]peer.AddrInfo, error) {
    peersData, err := os.ReadFile("known_peers.json")
    if err != nil {
        return nil, fmt.Errorf("failed to read known peers: %w", err)
    }

    var peers []peer.AddrInfo
    err = json.Unmarshal(peersData, &peers)
    if err != nil {
        return nil, fmt.Errorf("failed to unmarshal known peers: %w", err)
    }

    return peers, nil
}
type discoveryNotifee struct {
	PeerChan chan peer.AddrInfo
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.PeerChan <- pi
}

func (dht *DHT) discoverLocalPeers() ([]peer.AddrInfo, error) {
    n := &discoveryNotifee{}
	n.PeerChan = make(chan peer.AddrInfo)

    service := mdns.NewMdnsService(dht.Host, "discover_service", n)
    if err := service.Start(); err != nil {
		panic(err)
	}

    var peers []peer.AddrInfo
    for peerInfo := range n.PeerChan {
        peers = append(peers, peerInfo)
    }

    return peers, nil
}

func (dht *DHT) queryDNSSeeds() ([]peer.AddrInfo, error) {
    dnsSeeds := []string{
        "/dnsaddr/bootstrap.libp2p.io",
        "/dnsaddr/bootstrap.filecoin.io",
    }

    var peers []peer.AddrInfo
    for _, seed := range dnsSeeds {
        addr, err := multiaddr.NewMultiaddr(seed)
        if err != nil {
            dht.logger.Printf("Invalid DNS seed address %s: %v", seed, err)
            continue
        }

        peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
        if err != nil {
            dht.logger.Printf("Failed to get peer info from DNS seed %s: %v", seed, err)
            continue
        }

        peers = append(peers, *peerInfo)
    }

    return peers, nil
}

func (dht *DHT) saveKnownPeers(peers []peer.AddrInfo) error {
    peersData, err := json.Marshal(peers)
    if err != nil {
        return fmt.Errorf("failed to marshal known peers: %w", err)
    }

    err = os.WriteFile("known_peers.json", peersData, 0644)
    if err != nil {
        return fmt.Errorf("failed to save known peers: %w", err)
    }

    return nil
}