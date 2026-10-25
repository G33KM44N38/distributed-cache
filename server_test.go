package main

import (
	"bytes"
	"cache/cache"
	"cache/client"
	"cache/proto"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestServerDefaults(t *testing.T) {
	s := NewServer(ServerOpts{}, cache.New())
	if s.ListenAddr != "127.0.0.1:3000" || s.ReadTimeout != 5*time.Second {
		t.Fatalf("unexpected defaults: %+v", s.ServerOpts)
	}
	s = NewServer(ServerOpts{ListenAddr: ":3000", ReadTimeout: time.Second}, cache.New())
	if s.ListenAddr != ":3000" || s.ReadTimeout != time.Second {
		t.Fatalf("explicit options ignored: %+v", s.ServerOpts)
	}
}

func startPipeServer(t *testing.T, s *Server) (net.Conn, <-chan struct{}) {
	t.Helper()
	server, peer := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(server)
	}()
	t.Cleanup(func() {
		peer.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server connection did not close")
		}
	})
	if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	return peer, done
}

func TestServerRejectsInvalidCommandWithoutCacheWrite(t *testing.T) {
	var invalid bytes.Buffer
	invalid.WriteByte(byte(proto.CmdSet))
	binary.Write(&invalid, binary.LittleEndian, int32(-1))
	valid := (&proto.CommandSet{Key: []byte("key"), Value: []byte("value")}).Bytes()
	for _, frame := range [][]byte{invalid.Bytes(), valid[:len(valid)-1]} {
		c := cache.New()
		peer, done := startPipeServer(t, NewServer(ServerOpts{}, c))
		if _, err := peer.Write(frame); err != nil {
			t.Fatal(err)
		}
		peer.Close()
		<-done
		if c.Has([]byte("key")) {
			t.Fatal("invalid command changed cache")
		}
	}
}

func TestServerReadTimeout(t *testing.T) {
	for _, frame := range [][]byte{nil, {byte(proto.CmdSet), 1}} {
		peer, done := startPipeServer(t, NewServer(ServerOpts{ReadTimeout: 30 * time.Millisecond}, cache.New()))
		if len(frame) > 0 {
			if _, err := peer.Write(frame); err != nil {
				t.Fatal(err)
			}
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("idle or partial command was not timed out")
		}
	}
}

func TestServerProcessesConsecutiveSetsInOrder(t *testing.T) {
	c := cache.New()
	peer, done := startPipeServer(t, NewServer(ServerOpts{}, c))
	for _, value := range []string{"first", "last"} {
		frame := (&proto.CommandSet{Key: []byte("key"), Value: []byte(value)}).Bytes()
		if _, err := peer.Write(frame); err != nil {
			t.Fatal(err)
		}
	}
	peer.Close()
	<-done
	got, err := c.Get([]byte("key"))
	if err != nil || string(got) != "last" {
		t.Fatalf("got (%q, %v), want last", got, err)
	}
}

func TestTCPSetRoundTrip(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	c := cache.New()
	s := NewServer(ServerOpts{}, c)
	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			s.handleConn(conn)
		}
		done <- err
	}()
	peer, err := client.New(ln.Addr().String(), client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if _, err := peer.Set(context.Background(), []byte("key"), []byte("value"), 0); err != nil {
		t.Fatal(err)
	}
	peer.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("TCP connection did not complete")
	}
	got, err := c.Get([]byte("key"))
	if err != nil || string(got) != "value" {
		t.Fatalf("TCP SET got (%q, %v), want value", got, err)
	}
}
