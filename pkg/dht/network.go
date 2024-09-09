package maindht

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
)


func (dht *DHT) JoinNetwork() error {

    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Minute)
    defer cancel()

	knowPeers, err := dht.loadKnownPeers()
	if err != nil {
		return err
	}
	if len(knowPeers)>0 {
		return dht.Bootstrap(knowPeers)
	}

	localPeers, err := dht.discoverLocalPeers(ctx)
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


func (dht *DHT) discoverLocalPeers(ctx context.Context) ([]peer.AddrInfo, error) {
    notifee := &discoveryNotifee{
        PeerChan: make(chan peer.AddrInfo),
    }

    service := mdns.NewMdnsService(dht.Host, "your-service-name", notifee)
    if err := service.Start(); err != nil {
        return nil, fmt.Errorf("failed to start mDNS service: %w", err)
    }
    defer service.Close()

    var peers []peer.AddrInfo
    var mu sync.Mutex
    discoveryTimeout := time.After(30 * time.Second)

    for {
        select {
        case peerInfo := <-notifee.PeerChan:
            mu.Lock()
            peers = append(peers, peerInfo)
            mu.Unlock()
        case <-discoveryTimeout:
            return peers, nil
        case <-ctx.Done():
            return peers, ctx.Err()
        }
    }
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