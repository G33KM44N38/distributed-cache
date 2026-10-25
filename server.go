package main

import (
	"cache/cache"
	"cache/proto"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const (
	defaultListenAddr  = "127.0.0.1:3000"
	defaultReadTimeout = 5 * time.Second
)

type ServerOpts struct {
	ListenAddr  string
	IsLeader    bool
	LeaderAddr  string
	ReadTimeout time.Duration
}
type Server struct {
	ServerOpts
	cache cache.Cacher
}

func NewServer(opts ServerOpts, c cache.Cacher) *Server {
	if opts.ListenAddr == "" {
		opts.ListenAddr = defaultListenAddr
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = defaultReadTimeout
	}
	return &Server{
		ServerOpts: opts,
		cache:      c,
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen error: %s", err)
	}

	log.Printf("Server starting on port [%s]\n", s.ListenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accpet error: %s\n", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	fmt.Println("connection made:", conn.RemoteAddr())

	for {
		if err := conn.SetReadDeadline(time.Now().Add(s.ReadTimeout)); err != nil {
			log.Println("set read deadline error:", err)
			return
		}
		cmd, err := proto.ParseCommand(conn)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Println("parse command error:", err)
			break
		}

		s.handleCommand(conn, cmd)
	}
	fmt.Println("connection closed:", conn.RemoteAddr())
}

func (s *Server) handleCommand(conn net.Conn, cmd any) {
	switch v := cmd.(type) {
	case *proto.CommandSet:
		s.handleSetCommand(conn, v)
	case *proto.CommandGet:
	}
}

func (s *Server) handleSetCommand(conn net.Conn, cmd *proto.CommandSet) error {
	log.Printf("SET %s to %s", cmd.Key, cmd.Value)
	return s.cache.Set(cmd.Key, cmd.Value, time.Duration(cmd.TTL))
}
