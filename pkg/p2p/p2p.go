package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	blockchains "nebula/internal/blockChains"
	"os"
	"os/signal"
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
	host host.Host
}

func (p2pn *P2pNetwork) Network() {
	ctx := context.Background()

	fileBLockchain := blockchains.InitializeFileBlockchain()

	//creat new hote
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
		),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
	)

	if err != nil {
		log.Fatal(err)
	}
	p2pn.host = h

	h.SetStreamHandler("/p2p/1.0.0", func(s network.Stream) {
		handleFileStream(s, &fileBLockchain)
	})

	h.SetStreamHandler("/p2p/1.0.0", handelStream)
	for _, addr := range h.Addrs() {
		fmt.Printf("Adresse d'écoute: %s\n", addr)
	}
	fmt.Printf("ID de pair: %s\n", h.ID())

	if len(os.Args) > 1 {
		targetAddr, _ := multiaddr.NewMultiaddr(os.Args[1])
		peerinfo, _ := peer.AddrInfoFromP2pAddr(targetAddr)
		h.Peerstore().AddAddrs(peerinfo.ID, peerinfo.Addrs, peerstore.PermanentAddrTTL)
		if err := h.Connect(ctx, *peerinfo); err != nil {
			log.Println("Erreur lors de la connexion:", err)
			return
		}
		sendMessage(h, *peerinfo, "Bonjour depuis "+h.ID().String())
	}

	go func() {
		for {
			time.Sleep(10 * time.Second)
			displayTransactions(fileBLockchain)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	p2pn.Stop()
	//select {}
}

func (p2pn *P2pNetwork) Stop() {
	fmt.Println("Stopping P2P network...")
	if err := p2pn.host.Close(); err != nil {
		log.Println("Error closing the host:", err)
	} else {
		fmt.Println("P2P network stopped.")
	}
}

func handelStream(s network.Stream) {
	fmt.Println("new flux of :", s.Conn().RemotePeer())
	buf := make([]byte, 1024)
	n, err := s.Read(buf)
	if err != nil {
		log.Println("Erreur lors de la lecture du flux:", err)
		return
	}
	fmt.Printf("Reçu: %s\n", string(buf[:n]))
	s.Close()
}

func sendMessage(h host.Host, target peer.AddrInfo, message string) {
	s, err := h.NewStream(context.Background(), target.ID, "/p2p/1.0.0")
	if err != nil {
		log.Println("Erreur lors de la création du flux:", err)
		return
	}

	_, err = s.Write([]byte(message))
	if err != nil {
		log.Println("Erreur lors de l'envoi du message:", err)
		s.Close()
		return
	}
	s.Close()
}

func sendFileBLock(h host.Host, target peer.AddrInfo, fileBlock blockchains.FileBlock) {
	s, err := h.NewStream(context.Background(), target.ID, "/p2p/1.0.0")
	if err != nil {
		log.Println("Erreur lors de la création du flux:", err)
		return
	}
	encoder := json.NewEncoder(s)
	err = encoder.Encode(fileBlock)
}

func handleFileStream(s network.Stream, fileBlockchain *[]blockchains.FileBlock) {
	var newFileBlock blockchains.FileBlock
	decoder := json.NewDecoder(s)
	err := decoder.Decode(&newFileBlock)
	if err != nil {
		log.Println("Failed to decode new file block:", err)
		return
	}
	blockchains.AddFileBlock(fileBlockchain, newFileBlock)
	fmt.Println("New file block added to the blockchain:", newFileBlock)
	displayTransactions(*fileBlockchain)
	s.Close()
}

func displayTransactions(blockchain []blockchains.FileBlock) {
	fmt.Println("Current Blockchain State:")
	for _, block := range blockchain {
		fmt.Printf("Block Index: %d\n", block.Index)
		fmt.Printf("Timestamp: %s\n", block.Timestamp)
		fmt.Printf("Previous Hash: %s\n", block.PrevHash)
		fmt.Printf("Hash: %s\n", block.Hash)
		fmt.Println("Files:")
		for _, file := range block.Files {
			fmt.Printf("  File ID: %s\n", file.FileId)
			fmt.Printf("  File Name: %s\n", file.FileName)
			fmt.Printf("  File Size: %d\n", file.FileSize)
			fmt.Printf("  Owner: %s\n", file.Owner)
		}
		fmt.Println()
	}
}
