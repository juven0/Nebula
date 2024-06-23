package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	ctx := context.Background()
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

	h.SetStreamHandler("/p2p/1.0.0", func(s network.Stream) {
		fmt.Println("new flux of :", s.Conn().RemotePeer())
		defer s.Close()
	})

	// set personnalised protocole
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

	select {}

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
