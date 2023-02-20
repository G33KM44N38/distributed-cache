package main

import (
	"cache/cache"
	"flag"
	"log"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", ":3000")
	if err != nil {
		log.Fatal(err)
	}
	_, err = conn.Write([]byte("SET Foo Bar 4000000000000"))
	if err != nil {
		log.Fatal(err)
	}

	select {}

	return
	var (
		ListenAddr = flag.String("listenaddr", ":3000", "listen address of ther server")
		leaderAddr = flag.String("leaderaddr", "", "listen address of ther leader")
	)
	flag.Parse()
	opts := ServerOpts{
		ListenAddr: *ListenAddr,
		IsLeader:   len(*leaderAddr) == 0,
		LeaderAddr: *leaderAddr,
	}

	server := NewServer(opts, cache.New())
	server.Start()
}
