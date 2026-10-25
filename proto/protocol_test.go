package proto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
)

func TestParseRejectsNegativeLength(t *testing.T) {
	for _, command := range []Command{CmdGet, CmdSet} {
		t.Run(fmt.Sprint(command), func(t *testing.T) {
			var frame bytes.Buffer
			frame.WriteByte(byte(command))
			binary.Write(&frame, binary.LittleEndian, int32(-1))
			cmd, err := ParseCommand(&frame)
			if err == nil || cmd != nil {
				t.Fatalf("got (%v, %v), want nil command and error", cmd, err)
			}
		})
	}
}

// Invalid lengths must be rejected from the header alone, before reading a body.
type headerOnlyReader struct {
	*bytes.Reader
}

func (r headerOnlyReader) Read(p []byte) (int, error) {
	if r.Len() == 0 {
		panic("attempted body read after invalid length")
	}
	return r.Reader.Read(p)
}

func TestParseRejectsInvalidLengthsBeforeBody(t *testing.T) {
	for _, field := range []string{"get key", "set key", "set value"} {
		limit := int32(MaxKeySize)
		if field == "set value" {
			limit = MaxValueSize
		}
		for _, size := range []int32{-1, -2147483648, limit + 1, 2147483647} {
			t.Run(fmt.Sprintf("%s/%d", field, size), func(t *testing.T) {
				var frame bytes.Buffer
				command := CmdSet
				if field == "get key" {
					command = CmdGet
				}
				frame.WriteByte(byte(command))
				if field == "set value" {
					binary.Write(&frame, binary.LittleEndian, int32(0))
				}
				binary.Write(&frame, binary.LittleEndian, size)
				cmd, err := ParseCommand(headerOnlyReader{bytes.NewReader(frame.Bytes())})
				if cmd != nil || err == nil {
					t.Fatalf("got (%v, %v), want nil command and error", cmd, err)
				}
			})
		}
	}
}

func TestParseSizeBoundaries(t *testing.T) {
	for _, size := range []int{0, MaxKeySize - 1, MaxKeySize} {
		cmd := &CommandGet{Key: bytes.Repeat([]byte{'k'}, size)}
		parsed, err := ParseCommand(bytes.NewReader(cmd.Bytes()))
		if err != nil || !reflect.DeepEqual(cmd, parsed) {
			t.Fatalf("GET key size %d failed: %v", size, err)
		}
	}
	for _, size := range []int{0, MaxValueSize - 1, MaxValueSize} {
		cmd := &CommandSet{Key: bytes.Repeat([]byte{'k'}, MaxKeySize), Value: bytes.Repeat([]byte{'v'}, size), TTL: 2}
		parsed, err := ParseCommand(bytes.NewReader(cmd.Bytes()))
		if err != nil || !reflect.DeepEqual(cmd, parsed) {
			t.Fatalf("SET value size %d failed: %v", size, err)
		}
	}
}

type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestParsePreservesReaderErrors(t *testing.T) {
	failure := errors.New("read failed")
	frame := (&CommandSet{Key: []byte("key"), Value: []byte("value"), TTL: 2}).Bytes()
	for n := 0; n < len(frame); n++ {
		r := io.MultiReader(bytes.NewReader(frame[:n]), failingReader{failure})
		cmd, err := ParseCommand(r)
		if cmd != nil || !errors.Is(err, failure) {
			t.Fatalf("reader error at %d: got (%v, %v)", n, cmd, err)
		}
	}
}

func TestParseConsecutiveCommands(t *testing.T) {
	first := &CommandSet{Key: []byte("key"), Value: []byte("value"), TTL: 2}
	second := &CommandGet{Key: []byte("key")}
	r := bytes.NewReader(append(first.Bytes(), second.Bytes()...))
	for _, want := range []any{first, second} {
		got, err := ParseCommand(r)
		if err != nil || !reflect.DeepEqual(want, got) {
			t.Fatalf("got (%v, %v), want %v", got, err, want)
		}
	}
	if r.Len() != 0 {
		t.Fatal("unconsumed command bytes")
	}
}

func TestParseUnknownCommand(t *testing.T) {
	cmd, err := ParseCommand(bytes.NewReader([]byte{255}))
	if cmd != nil || err == nil {
		t.Fatalf("got (%v, %v), want nil command and error", cmd, err)
	}
}

func TestParseRejectsTruncatedCommands(t *testing.T) {
	frames := [][]byte{
		(&CommandGet{Key: []byte("key")}).Bytes(),
		(&CommandSet{Key: []byte("key"), Value: []byte("value"), TTL: 2}).Bytes(),
	}
	for _, frame := range frames {
		for n := 0; n < len(frame); n++ {
			cmd, err := ParseCommand(bytes.NewReader(frame[:n]))
			if cmd != nil || (!errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF)) {
				t.Errorf("command %d truncated at %d: got (%v, %v)", frame[0], n, cmd, err)
			}
		}
	}
}

func TestParseSetCommand(t *testing.T) {
	cmd := &CommandSet{
		Key:   []byte("Foo"),
		Value: []byte("Bar"),
		TTL:   2,
	}
	r := bytes.NewReader(cmd.Bytes())
	pcmd, err := ParseCommand(r)
	if err != nil || !reflect.DeepEqual(cmd, pcmd) {
		t.Fatalf("got (%v, %v), want %v", pcmd, err, cmd)
	}
}

func TestParseGetCommand(t *testing.T) {
	cmd := &CommandGet{
		Key: []byte("Foo"),
	}
	r := bytes.NewReader(cmd.Bytes())
	pcmd, err := ParseCommand(r)
	if err != nil || !reflect.DeepEqual(cmd, pcmd) {
		t.Fatalf("got (%v, %v), want %v", pcmd, err, cmd)
	}
}

func BenchmarkParseCommand(b *testing.B) {
	cmd := &CommandSet{
		Key:   []byte("Foo"),
		Value: []byte("Bar"),
		TTL:   2,
	}
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(cmd.Bytes())
		ParseCommand(r)
	}
}
