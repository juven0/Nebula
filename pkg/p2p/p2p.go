package p2p

import (
	"context"
	"fmt"
	"log"
	blockchains "nebula/internal/blockChains"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

type P2pNetwork struct {
	Host    host.Host
	Context context.Context
	cancel  context.CancelFunc
}

func (p2pn *P2pNetwork) Init() error {
	p2pn.Context, p2pn.cancel = context.WithCancel(context.Background())

	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
		),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
	)
	if err != nil {
		return fmt.Errorf("failed to create libp2p host: %w", err)
	}

	p2pn.Host = h
	return nil
}

func (p2pn *P2pNetwork) Connect(address string) error {
	peerAddr, err := multiaddr.NewMultiaddr(address)
	if err != nil {
		return fmt.Errorf("invalid multiaddress: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
	if err != nil {
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	if err := p2pn.Host.Connect(p2pn.Context, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	log.Printf("Successfully connected to: %s", peerInfo)
	return nil
}

func (p2pn *P2pNetwork) Network() {
	fileBlockchain := blockchains.InitializeFileBlockchain()

	p2pn.Host.SetStreamHandler("/p2p/1.0.0", func(s network.Stream) {
		blockchains.HandleFileStream(s, &fileBlockchain)
	})

	p2pn.Host.SetStreamHandler("/p2p/1.0.0", handleStream)

	for _, addr := range p2pn.Host.Addrs() {
		log.Printf("Listening address: %s", addr)
	}
	log.Printf("Peer ID: %s", p2pn.Host.ID())

	if len(os.Args) > 1 {
		if err := p2pn.connectToPeer(os.Args[1]); err != nil {
			log.Printf("Failed to connect to peer: %v", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go p2pn.monitorPeers(&wg)
	// go p2pn.monitorBlockchain(&wg, fileBlockchain)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	p2pn.Stop()
	wg.Wait()
}

func (p2pn *P2pNetwork) Stop() {
	log.Println("Stopping P2P network...")
	p2pn.cancel()
	if err := p2pn.Host.Close(); err != nil {
		log.Printf("Error closing the host: %v", err)
	} else {
		log.Println("P2P network stopped.")
	}
}

func handleStream(s network.Stream) {
	defer s.Close()

	log.Printf("New stream from: %s", s.Conn().RemotePeer())
	buf := make([]byte, 1024)
	n, err := s.Read(buf)
	if err != nil {
		log.Printf("Error reading from stream: %v", err)
		return
	}
	log.Printf("Received: %s", string(buf[:n]))
}

func sendMessage(h host.Host, target peer.AddrInfo, message string) error {
	s, err := h.NewStream(context.Background(), target.ID, "/p2p/1.0.0")
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer s.Close()

	_, err = s.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}

func (p2pn *P2pNetwork) connectToPeer(targetAddr string) error {
	maddr, err := multiaddr.NewMultiaddr(targetAddr)
	if err != nil {
		return fmt.Errorf("invalid multiaddress: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	p2pn.Host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
	if err := p2pn.Host.Connect(p2pn.Context, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	return sendMessage(p2pn.Host, *peerInfo, fmt.Sprintf("Hello from %s", p2pn.Host.ID()))
}

func (p2pn *P2pNetwork) monitorPeers(wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			peers := p2pn.Host.Network().Peers()
			log.Printf("Number of connected peers: %d", len(peers))
			for _, p := range peers {
				addrs := p2pn.Host.Peerstore().Addrs(p)
				log.Printf("Peer: %s, Addresses: %v", p, addrs)
			}
		case <-p2pn.Context.Done():
			return
		}
	}
}

// func (p2pn *P2pNetwork) monitorBlockchain(wg *sync.WaitGroup, blockchain blockchains.FileBlockchain) {
// 	defer wg.Done()
// 	ticker := time.NewTicker(10 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			blockchains.DisplayTransactions(blockchain)
// 		case <-p2pn.Context.Done():
// 			return
// 		}
// 	}
// }
