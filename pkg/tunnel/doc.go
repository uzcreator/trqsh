// Package tunnel provides the multiplexed transport between agent and edge:
// a QUIC-first dialer with TCP+yamux fallback, plus the Session, Stream, and
// Listener abstractions.
//
// Implemented in Part 01. This is a FROZEN contract — see section 7 of
// plan/00-ARCHITECTURE.md and plan/01-protocol-transport.md.
package tunnel
