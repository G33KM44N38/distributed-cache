package client

import (
	"cache/proto"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestSetRoundTrip(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	c := &Client{conn: local}
	defer c.Close()
	done := make(chan error, 1)
	go func() {
		_, err := c.Set(context.Background(), []byte("key"), []byte("value"), 2)
		done <- err
	}()
	remote.SetReadDeadline(time.Now().Add(time.Second))
	got, err := proto.ParseCommand(remote)
	if err != nil {
		t.Fatal(err)
	}
	cmd := got.(*proto.CommandSet)
	if string(cmd.Key) != "key" || string(cmd.Value) != "value" || cmd.TTL != 2 {
		t.Fatalf("unexpected command: %+v", cmd)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSetRejectsOversizedFields(t *testing.T) {
	c := &Client{} // Must fail before touching the connection or encoding a frame.
	for _, fields := range [][2][]byte{
		{make([]byte, proto.MaxKeySize+1), nil},
		{nil, make([]byte, proto.MaxValueSize+1)},
	} {
		if _, err := c.Set(context.Background(), fields[0], fields[1], 0); err == nil {
			t.Fatal("oversized field accepted")
		}
	}
}

func TestSetWriteDeadlineClosesConnection(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	c := &Client{conn: local}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := c.Set(ctx, []byte("key"), []byte("value"), 0)
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("expected timeout, got %v", err)
	}
	remote.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := remote.Write([]byte{1}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("failed write left connection open: %v", err)
	}
}

func TestSetCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&Client{}).Set(ctx, []byte("k"), nil, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}
}

type writeConn struct {
	net.Conn
	deadline    time.Time
	deadlineErr error
	writeErr    error
	short       bool
	closed      bool
	writes      int
}

func (c *writeConn) SetWriteDeadline(d time.Time) error {
	c.deadline = d
	return c.deadlineErr
}

func (c *writeConn) Write(p []byte) (int, error) {
	c.writes++
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	if c.short {
		return len(p) - 1, nil
	}
	return len(p), nil
}

func (c *writeConn) Close() error { c.closed = true; return nil }

func TestSetUsesBoundedDeadline(t *testing.T) {
	for _, duration := range []time.Duration{time.Second, time.Hour} {
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		conn := &writeConn{}
		before := time.Now()
		_, err := (&Client{conn: conn}).Set(ctx, []byte("key"), nil, 0)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if conn.deadline.After(time.Now().Add(networkTimeout)) || !conn.deadline.After(before) {
			t.Fatalf("unbounded or expired deadline: %v", conn.deadline)
		}
		if duration == time.Second {
			want, _ := ctx.Deadline()
			if !conn.deadline.Equal(want) {
				t.Fatal("context deadline ignored")
			}
		}
	}
}

func TestSetHandlesWriteErrors(t *testing.T) {
	failure := errors.New("connection failed")
	for _, conn := range []*writeConn{{deadlineErr: failure}, {writeErr: failure}, {short: true}} {
		_, err := (&Client{conn: conn}).Set(context.Background(), []byte("key"), nil, 0)
		want := failure
		if conn.short {
			want = io.ErrShortWrite
		}
		if !errors.Is(err, want) {
			t.Fatalf("got %v, want %v", err, want)
		}
		if conn.deadlineErr != nil && conn.writes != 0 {
			t.Fatal("write attempted after deadline error")
		}
		if conn.deadlineErr == nil && !conn.closed {
			t.Fatal("connection reused after write error")
		}
	}
}

func TestNewReturnsDialError(t *testing.T) {
	c, err := New("127.0.0.1:invalid", Options{})
	if c != nil || err == nil {
		t.Fatalf("got (%v, %v), want dial error", c, err)
	}
}
