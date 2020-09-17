package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	noise "github.com/libp2p/go-libp2p-noise"
	yamux "github.com/libp2p/go-libp2p-yamux"
	ma "github.com/multiformats/go-multiaddr"
)

const BUFFER_SIZE = 128_000
const PROTOCOL_NAME = "/perf/0.1.0"

var MSG = make([]byte, BUFFER_SIZE)

func main() {
	target := flag.String("server-address", "", "")
	flag.Parse()

	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		panic(err)
	}

	opts := []libp2p.Option{
		libp2p.Security(noise.ID, noise.New),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 0)),
		libp2p.Identity(priv),
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
	}

	basicHost, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		panic(err)
	}

	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", basicHost.ID().Pretty()))
	addr := basicHost.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)

	basicHost.SetStreamHandler(PROTOCOL_NAME, func(s network.Stream) {
		if err := handleIncomingPerfRun(s); err != nil {
			log.Println(err)
			s.Reset()
		} else {
			s.Close()
		}
	})

	// In case binary runs as a server.
	if *target == "" {
		log.Printf("Now run \"./go-libp2p-perf --server-address %s\" on a different terminal.\n", fullAddr)
		log.Println("Listening for connections.")
		select {} // hang forever
	}

	// The following code extracts target's the peer ID from the
	// given multiaddress
	ipfsaddr, err := ma.NewMultiaddr(*target)
	if err != nil {
		log.Fatalln(err)
	}

	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		log.Fatalln(err)
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		log.Fatalln(err)
	}

	// Decapsulate the /ipfs/<peerID> part from the target
	// /ip4/<a.b.c.d>/ipfs/<peer> becomes /ip4/<a.b.c.d>
	targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", pid))
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)

	// We have a peer ID and a targetAddr so we add it to the peerstore
	// so LibP2P knows how to contact it
	basicHost.Peerstore().AddAddr(peerid, targetAddr, peerstore.PermanentAddrTTL)

	s, err := basicHost.NewStream(context.Background(), peerid, PROTOCOL_NAME)
	if err != nil {
		log.Fatalln(err)
	}

	start := time.Now()
	transfered := 0
	for time.Now().Sub(start) < 10*time.Second {
		_, err = s.Write(MSG)
		if err != nil {
			log.Fatalln(err)
		}
		transfered += BUFFER_SIZE
	}

	printRun(start, transfered)
}

func handleIncomingPerfRun(s network.Stream) error {
	var err error
	start := time.Now()
	transfered := 0
	buf := make([]byte, BUFFER_SIZE)

	for err == nil {
		_, err = io.ReadFull(s, buf)
		transfered += BUFFER_SIZE
	}

	printRun(start, transfered)

	return err
}

func printRun(start time.Time, transfered int) {
	fmt.Printf(
		"Interval \tTransfer\tBandwidth\n0s - %.2f s \t%d MBytes\t %.2f MBit/s\n",
		time.Now().Sub(start).Seconds(),
		transfered/1000/1000,
		float64(transfered/1000/1000*8)/time.Now().Sub(start).Seconds(),
	)
}
