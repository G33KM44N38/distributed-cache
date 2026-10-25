package client

import (
	"cache/proto"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const networkTimeout = 5 * time.Second

type (
	Options struct{}
	Client  struct {
		conn net.Conn
		mu   sync.Mutex
	}
)

func New(endpoint string, opts Options) (*Client, error) {
	conn, err := net.DialTimeout("tcp", endpoint, networkTimeout)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn: conn,
	}, nil
}

func (c *Client) Set(ctx context.Context, key, value []byte, ttl int) (any, error) {
	if len(key) > proto.MaxKeySize || len(value) > proto.MaxValueSize {
		return nil, fmt.Errorf("key or value exceeds protocol size limit")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(networkTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return nil, err
	}
	cmd := &proto.CommandSet{
		Key:   key,
		Value: value,
		TTL:   ttl,
	}
	data := cmd.Bytes()
	n, err := c.conn.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		// A partial frame cannot be reused for another command.
		c.conn.Close()
		return nil, err
	}

	return nil, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
