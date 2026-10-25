package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type Command byte

const (
	CmdNonce Command = iota
	CmdSet
	CmdGet
	CmdDet
)

const (
	MaxKeySize   = 64 << 10
	MaxValueSize = 1 << 20
)

type CommandSet struct {
	Key   []byte
	Value []byte
	TTL   int
}

func (c *CommandSet) Bytes() []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, CmdSet)

	keyLen := int32(len(c.Key))

	binary.Write(buf, binary.LittleEndian, keyLen)
	binary.Write(buf, binary.LittleEndian, c.Key)

	valueLen := int32(len(c.Value))
	binary.Write(buf, binary.LittleEndian, valueLen)
	binary.Write(buf, binary.LittleEndian, c.Value)

	binary.Write(buf, binary.LittleEndian, int32(c.TTL))

	return buf.Bytes()
}

func ParseCommand(r io.Reader) (any, error) {
	var cmd Command
	if err := binary.Read(r, binary.LittleEndian, &cmd); err != nil {
		return nil, err
	}
	switch cmd {
	case CmdSet:
		parsed, err := parseSetCommand(r)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case CmdGet:
		parsed, err := parseGetCommand(r)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("invalid command")
	}
}

type CommandGet struct {
	Key []byte
}

func (c *CommandGet) Bytes() []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, CmdGet)

	keyLen := int32(len(c.Key))

	binary.Write(buf, binary.LittleEndian, keyLen)
	binary.Write(buf, binary.LittleEndian, c.Key)

	return buf.Bytes()
}

func parseSetCommand(r io.Reader) (*CommandSet, error) {
	cmd := &CommandSet{}
	var err error
	if cmd.Key, err = readField(r, "key", MaxKeySize); err != nil {
		return nil, err
	}
	if cmd.Value, err = readField(r, "value", MaxValueSize); err != nil {
		return nil, err
	}

	var ttl int32
	if err := binary.Read(r, binary.LittleEndian, &ttl); err != nil {
		return nil, fmt.Errorf("read ttl: %w", err)
	}
	cmd.TTL = int(ttl)
	return cmd, nil
}

func parseGetCommand(r io.Reader) (*CommandGet, error) {
	key, err := readField(r, "key", MaxKeySize)
	if err != nil {
		return nil, err
	}
	return &CommandGet{Key: key}, nil
}

func readField(r io.Reader, name string, limit int32) ([]byte, error) {
	var size int32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return nil, fmt.Errorf("read %s length: %w", name, err)
	}
	if size < 0 || size > limit {
		return nil, fmt.Errorf("invalid %s length %d (limit %d)", name, size, limit)
	}
	data := make([]byte, int(size))
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return data, nil
}
