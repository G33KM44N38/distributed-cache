package main

import (
	"cache/cache"
	"cache/client"
	"context"
	"flag"
	"log"
)

func main() {
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

	// go func() {
	// 	time.Sleep(time.Second * 2)
	// 	client, err := client.New(":3000", client.Options{})
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	for i := 0; i < 10; i++ {
	// 		SendCommand(client)
	// 	}
	// 	client.Close()
	// 	time.Sleep(time.Millisecond * 200)
	// }()

	server := NewServer(opts, cache.New())
	server.Start()
}

func SendCommand(c *client.Client) {
	_, err := c.Set(context.Background(), []byte("gg"), []byte("Kylian"), 0)
	if err != nil {
		log.Fatal(err)
	}
}
