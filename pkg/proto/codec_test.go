package proto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	cases := []*Envelope{
		{Msg: &Envelope_Ping{Ping: &Ping{TsUnixMs: 1234567890}}},
		{Msg: &Envelope_Pong{Pong: &Pong{TsUnixMs: 987654321}}},
		{Msg: &Envelope_Error{Error: NewError(CodeQuotaTunnels, "too many tunnels")}},
		{Msg: &Envelope_Hello{Hello: &Hello{}}},
	}
	for _, want := range cases {
		var buf bytes.Buffer
		if err := WriteMsg(&buf, want); err != nil {
			t.Fatalf("WriteMsg: %v", err)
		}
		got, err := ReadMsg(&buf)
		if err != nil {
			t.Fatalf("ReadMsg: %v", err)
		}
		if !proto.Equal(want, got) {
			t.Errorf("round-trip mismatch:\n want %v\n  got %v", want, got)
		}
		if buf.Len() != 0 {
			t.Errorf("expected buffer fully consumed, %d bytes left", buf.Len())
		}
	}
}

func TestStreamInitRoundTrip(t *testing.T) {
	want := &StreamInit{
		ClientTunnelId: "tnl_abc123",
		RemoteAddr:     "203.0.113.5:44012",
		Proto:          "https",
		Meta:           map[string]string{"sni": "abc.trqsh.uz", "alpn": "h2"},
	}
	var buf bytes.Buffer
	if err := WriteStreamInit(&buf, want); err != nil {
		t.Fatalf("WriteStreamInit: %v", err)
	}
	got, err := ReadStreamInit(&buf)
	if err != nil {
		t.Fatalf("ReadStreamInit: %v", err)
	}
	if !proto.Equal(want, got) {
		t.Errorf("round-trip mismatch:\n want %v\n  got %v", want, got)
	}
}

func TestConsecutiveFramesOnOneStream(t *testing.T) {
	var buf bytes.Buffer
	msgs := []*Envelope{
		{Msg: &Envelope_Ping{Ping: &Ping{TsUnixMs: 1}}},
		{Msg: &Envelope_Pong{Pong: &Pong{TsUnixMs: 2}}},
		{Msg: &Envelope_Ping{Ping: &Ping{TsUnixMs: 3}}},
	}
	for _, m := range msgs {
		if err := WriteMsg(&buf, m); err != nil {
			t.Fatalf("WriteMsg: %v", err)
		}
	}
	for i, want := range msgs {
		got, err := ReadMsg(&buf)
		if err != nil {
			t.Fatalf("ReadMsg[%d]: %v", i, err)
		}
		if !proto.Equal(want, got) {
			t.Errorf("frame %d mismatch: want %v got %v", i, want, got)
		}
	}
}

func TestWriteRejectsOversizeFrame(t *testing.T) {
	big := &Envelope{Msg: &Envelope_Error{Error: NewError(CodeInternal, strings.Repeat("x", MaxFrameSize+1))}}
	var buf bytes.Buffer
	err := WriteMsg(&buf, big)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("nothing should be written on oversize, got %d bytes", buf.Len())
	}
}

func TestReadRejectsOversizeLengthPrefix(t *testing.T) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], MaxFrameSize+1)
	// Only the header — a correct reader must reject before reading a body.
	_, err := ReadMsg(bytes.NewReader(hdr[:]))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}

func TestReadTruncatedFrame(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMsg(&buf, &Envelope{Msg: &Envelope_Ping{Ping: &Ping{TsUnixMs: 42}}}); err != nil {
		t.Fatalf("WriteMsg: %v", err)
	}
	// Chop the body in half; the reader should surface an unexpected EOF.
	truncated := buf.Bytes()[:buf.Len()-1]
	_, err := ReadMsg(bytes.NewReader(truncated))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
	}
}
