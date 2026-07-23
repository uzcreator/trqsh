package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/trqsh-uz/trqsh/pkg/proto"
)

var errPortUnavailable = errors.New("server: no ports available")

// portManager owns the public TCP/UDP port pool and the active listeners.
type portManager struct {
	min, max int

	mu      sync.Mutex
	usedTCP map[int]bool
	usedUDP map[int]bool
	tcp     map[int]net.Listener
	udp     map[int]*net.UDPConn
}

func newPortManager(min, max int) *portManager {
	return &portManager{
		min: min, max: max,
		usedTCP: make(map[int]bool),
		usedUDP: make(map[int]bool),
		tcp:     make(map[int]net.Listener),
		udp:     make(map[int]*net.UDPConn),
	}
}

func (pm *portManager) used(proto string) map[int]bool {
	if proto == "udp" {
		return pm.usedUDP
	}
	return pm.usedTCP
}

// reserve marks a port used. want != 0 asks for a specific port; 0 picks any
// free port in range.
func (pm *portManager) reserve(proto string, want int) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	used := pm.used(proto)
	if want != 0 {
		if want < pm.min || want > pm.max || used[want] {
			return 0, errPortUnavailable
		}
		used[want] = true
		return want, nil
	}
	for p := pm.min; p <= pm.max; p++ {
		if !used[p] {
			used[p] = true
			return p, nil
		}
	}
	return 0, errPortUnavailable
}

func (pm *portManager) setTCP(port int, ln net.Listener) {
	pm.mu.Lock()
	pm.tcp[port] = ln
	pm.mu.Unlock()
}

func (pm *portManager) setUDP(port int, uc *net.UDPConn) {
	pm.mu.Lock()
	pm.udp[port] = uc
	pm.mu.Unlock()
}

// release frees a port and closes its listener/conn if open.
func (pm *portManager) release(proto string, port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.used(proto), port)
	if proto == "udp" {
		if uc, ok := pm.udp[port]; ok {
			uc.Close()
			delete(pm.udp, port)
		}
		return
	}
	if ln, ok := pm.tcp[port]; ok {
		ln.Close()
		delete(pm.tcp, port)
	}
}

func (pm *portManager) closeAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, ln := range pm.tcp {
		ln.Close()
	}
	for _, uc := range pm.udp {
		uc.Close()
	}
	pm.tcp = make(map[int]net.Listener)
	pm.udp = make(map[int]*net.UDPConn)
}

// openPort reserves a port and starts the matching public listener.
func (s *Server) openPort(proto string, want int, bt *boundTunnel) (int, error) {
	if proto == "udp" {
		return s.openUDPPort(want, bt)
	}
	return s.openTCPPort(want)
}

func (s *Server) openTCPPort(want int) (int, error) {
	port, err := s.ports.reserve("tcp", want)
	if err != nil {
		return 0, err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		s.ports.release("tcp", port)
		return 0, err
	}
	s.ports.setTCP(port, ln)
	s.log.Info("tcp tunnel port up", "port", port)
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.acceptTCPTunnel(ln, port) }()
	return port, nil
}

func (s *Server) acceptTCPTunnel(ln net.Listener, port int) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return // listener closed on release
		}
		bt := s.hub.LookupPort("tcp", port)
		if bt == nil {
			c.Close()
			continue
		}
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.weldRawConn(c, bt, "tcp") }()
	}
}

// weldRawConn welds a raw public TCP conn to a fresh agent data stream.
func (s *Server) weldRawConn(c net.Conn, bt *boundTunnel, ptype string) {
	init := &proto.StreamInit{
		ClientTunnelId: bt.clientTunnelID,
		RemoteAddr:     c.RemoteAddr().String(),
		Proto:          ptype,
	}
	st, err := bt.session.openDataStream(context.Background(), init)
	if err != nil {
		s.metrics.Errors.WithLabelValues("open_stream").Inc()
		c.Close()
		return
	}
	s.metrics.StreamsOpened.WithLabelValues(ptype).Inc()
	up, down := rawJoin(c, c, st)
	s.metrics.countBytes(up, down)
	s.usage.record(bt.accountID, bt.clientTunnelID, up, down, 1)
}
