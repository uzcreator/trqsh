package server

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/trqsh-uz/trqsh/pkg/proto"
	"github.com/trqsh-uz/trqsh/pkg/tunnel"
)

// UDP datagrams are carried over a data stream with a uint16 big-endian length
// prefix per datagram. The agent MUST use the same framing when forwarding to
// and from the local UDP service.
const udpIdleTimeout = 60 * time.Second

type udpFlow struct {
	stream   tunnel.Stream
	writeMu  sync.Mutex
	lastSeen time.Time
}

func (s *Server) openUDPPort(want int, _ *boundTunnel) (int, error) {
	port, err := s.ports.reserve("udp", want)
	if err != nil {
		return 0, err
	}
	uc, err := net.ListenUDP("udp", &net.UDPAddr{Port: port})
	if err != nil {
		s.ports.release("udp", port)
		return 0, err
	}
	s.ports.setUDP(port, uc)
	s.log.Info("udp tunnel port up", "port", port)
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.serveUDPTunnel(uc, port) }()
	return port, nil
}

func (s *Server) serveUDPTunnel(uc *net.UDPConn, port int) {
	flows := make(map[string]*udpFlow)
	var mu sync.Mutex

	// Idle sweeper.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTicker(udpIdleTimeout / 2)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				now := time.Now()
				mu.Lock()
				for k, f := range flows {
					if now.Sub(f.lastSeen) > udpIdleTimeout {
						f.stream.Close()
						delete(flows, k)
					}
				}
				mu.Unlock()
			}
		}
	}()

	buf := make([]byte, 65535)
	for {
		n, addr, err := uc.ReadFromUDP(buf)
		if err != nil {
			return // conn closed on release
		}
		bt := s.hub.LookupPort("udp", port)
		if bt == nil {
			continue
		}
		key := addr.String()

		mu.Lock()
		f := flows[key]
		if f == nil {
			st, err := bt.session.openDataStream(context.Background(), &proto.StreamInit{
				ClientTunnelId: bt.clientTunnelID,
				RemoteAddr:     key,
				Proto:          "udp",
			})
			if err != nil {
				mu.Unlock()
				s.metrics.Errors.WithLabelValues("open_stream").Inc()
				continue
			}
			s.metrics.StreamsOpened.WithLabelValues("udp").Inc()
			f = &udpFlow{stream: st, lastSeen: time.Now()}
			flows[key] = f
			// Agent -> public reader for this flow.
			go s.udpReadBack(uc, addr, f, bt)
		}
		f.lastSeen = time.Now()
		mu.Unlock()

		payload := make([]byte, n)
		copy(payload, buf[:n])
		if err := writeUDPFrame(f, payload); err != nil {
			mu.Lock()
			delete(flows, key)
			mu.Unlock()
			f.stream.Close()
			continue
		}
		s.usage.record(bt.accountID, bt.clientTunnelID, int64(n), 0, 1)
	}
}

func writeUDPFrame(f *udpFlow, payload []byte) error {
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(payload)))
	f.writeMu.Lock()
	defer f.writeMu.Unlock()
	if _, err := f.stream.Write(hdr[:]); err != nil {
		return err
	}
	_, err := f.stream.Write(payload)
	return err
}

// udpReadBack reads framed datagrams from the agent stream and sends them to the
// public client.
func (s *Server) udpReadBack(uc *net.UDPConn, addr *net.UDPAddr, f *udpFlow, bt *boundTunnel) {
	var hdr [2]byte
	for {
		if _, err := io.ReadFull(f.stream, hdr[:]); err != nil {
			return
		}
		n := binary.BigEndian.Uint16(hdr[:])
		payload := make([]byte, n)
		if _, err := io.ReadFull(f.stream, payload); err != nil {
			return
		}
		if _, err := uc.WriteToUDP(payload, addr); err != nil {
			return
		}
		s.metrics.countBytes(0, int64(n))
		s.usage.record(bt.accountID, bt.clientTunnelID, 0, int64(n), 0)
	}
}
