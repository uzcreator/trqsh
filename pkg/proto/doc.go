// Package proto defines Rift's wire protocol: the length-prefixed Protobuf
// control messages (generated from proto/rift.proto) and the framed codec
// shared by the edge and the agent.
//
// This is a FROZEN contract — see section 6 of plan/00-ARCHITECTURE.md.
// Regenerate the .pb.go with `buf generate` (or `make proto`).
package proto
