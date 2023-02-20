package main

import (
	"cache/cache"
	"context"
	"fmt"
	"log"
	"net"
)

type ServerOpts struct {
	ListenAddr string
	IsLeader   bool
	LeaderAddr string
}
type Server struct {
	ServerOpts
	followers map[net.Conn]struct{}
	cache     cache.Cacher
}

func NewServer(opts ServerOpts, c cache.Cacher) *Server {
	return &Server{
		ServerOpts: opts,
		cache:      c,
		followers:  make(map[net.Conn]struct{}),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen error: %s", err)
	}

	log.Printf("Server starting on port [%s]\n", s.ListenAddr)

	if !s.IsLeader {
		go func() {
			conn, err := net.Dial("tcp", s.LeaderAddr)
			fmt.Println("Connected with leader:", s.LeaderAddr)
			if err != nil {
				log.Fatal(err)
			}
			s.handlConn(conn)
		}()
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accpet error: %s\n", err)
			continue
		}
		go s.handlConn(conn)

	}
}

func (s *Server) handlConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 2048)

	if s.IsLeader {
		s.followers[conn] = struct{}{}
	}

	fmt.Println("connection made:", conn.RemoteAddr())
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("conn read error: %s\n", err)
			break
		}

		go s.handleCommand(conn, buf[:n])
	}
}

func (s *Server) handleCommand(conn net.Conn, rawCmd []byte) {
	msg, err := parseMessage(rawCmd)
	if err != nil {
		fmt.Println("failed to parse command ", msg)
		conn.Write([]byte(err.Error()))
		// respond
		return
	}

	fmt.Printf("received command %s ", msg.Cmd)

	switch msg.Cmd {
	case CMDSet:
		err = s.handleSetCmd(conn, msg)
	case CMDGet:
		err = s.handleGetCmd(conn, msg)
	}

	if err != nil {
		fmt.Println("failed to handle command:", msg)
		conn.Write([]byte(err.Error()))
	}
}

func (s *Server) handleGetCmd(conn net.Conn, msg *Messsage) error {
	val, err := s.cache.Get(msg.Key)
	if err != nil {
		return err
	}
	_, err = conn.Write(val)

	return nil
}

func (s *Server) handleSetCmd(conn net.Conn, msg *Messsage) error {
	if err := s.cache.Set(msg.Key, msg.Value, msg.TTL); err != nil {
		return err
	}

	go s.sendToFollowers(context.TODO(), msg)
	return nil
}

func (s *Server) sendToFollowers(ctx context.Context, msg *Messsage) error {
	for conn := range s.followers {
		fmt.Println("forwarding key to follower")
		rawMsg := msg.ToBytes()
		_, err := conn.Write(rawMsg)
		fmt.Println("forwarding rawmsg to follower:", string(rawMsg))
		if err != nil {
			log.Println("write to followers error:", err)
			continue
		}

	}
	return nil
}
