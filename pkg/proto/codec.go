package proto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

// MaxFrameSize bounds a single length-prefixed control or stream-init frame,
// guarding against a malicious or corrupt length prefix.
const MaxFrameSize = 1 << 20 // 1 MiB

// ErrFrameTooLarge is returned when a frame's declared length exceeds MaxFrameSize.
var ErrFrameTooLarge = errors.New("rift/proto: frame exceeds MaxFrameSize")

// WriteMsg writes a length-prefixed Envelope: a uint32 big-endian length
// followed by the marshaled protobuf bytes.
func WriteMsg(w io.Writer, m *Envelope) error { return writeFramed(w, m) }

// ReadMsg reads a single length-prefixed Envelope written by WriteMsg.
func ReadMsg(r io.Reader) (*Envelope, error) {
	m := &Envelope{}
	if err := readFramed(r, m); err != nil {
		return nil, err
	}
	return m, nil
}

// WriteStreamInit writes the single StreamInit header frame that precedes the
// raw bytes on a data stream.
func WriteStreamInit(w io.Writer, s *StreamInit) error { return writeFramed(w, s) }

// ReadStreamInit reads the StreamInit header frame from the front of a data stream.
func ReadStreamInit(r io.Reader) (*StreamInit, error) {
	s := &StreamInit{}
	if err := readFramed(r, s); err != nil {
		return nil, err
	}
	return s, nil
}

func writeFramed(w io.Writer, m proto.Message) error {
	b, err := proto.Marshal(m)
	if err != nil {
		return fmt.Errorf("rift/proto: marshal: %w", err)
	}
	if len(b) > MaxFrameSize {
		return ErrFrameTooLarge
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(b)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	return nil
}

func readFramed(r io.Reader, m proto.Message) error {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > MaxFrameSize {
		return ErrFrameTooLarge
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return proto.Unmarshal(buf, m)
}
