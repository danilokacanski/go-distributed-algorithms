// Package link implements communication link abstractions from
// Cachin et al., Chapter 2, Section 2.4.
//
// Link abstractions model the reliability guarantees of the
// communication channel between processes. They form a hierarchy:
//
//	Unreliable → Fair-Loss → Stubborn → Perfect → Authenticated
//
// Each higher-level link is built by COMPOSING the lower-level one
// with additional mechanisms (retransmission, deduplication, authentication).
package link

import (
	"math/rand"

	"github.com/danilokacanski/da/week0203_basic_abstractions/process"
)

// Link is the interface for all communication link abstractions.
//
// In the formal model, a link provides two events:
//   - Send: process p requests to send message m to process q
//   - Deliver: message m is delivered to process q
//
// In our simulator, the link layer sits between the runtime scheduler
// and message delivery:
//  1. Send() is called when a process produces an outgoing message
//  2. Tick() is called each step to handle retransmissions
//  3. Receive() is called before delivering to check dedup/auth
type Link interface {
	// Send processes an outgoing message. Returns messages to enqueue.
	// May return:
	//   - empty slice: message was lost/dropped
	//   - single message: normal delivery
	//   - multiple messages: duplication occurred
	Send(msg process.Message, rng *rand.Rand) []process.Message

	// Tick is called each scheduler step. Returns any retransmissions.
	// Only meaningful for stubborn/higher links.
	Tick(step int, rng *rand.Rand) []process.Message

	// Receive checks if a message should be delivered.
	// Returns (processed_message, should_deliver).
	// Used for deduplication (perfect links) and authentication checks.
	Receive(msg process.Message) (process.Message, bool)
}

// ============================================================================
// UNRELIABLE LINK (baseline)
// ============================================================================

// UnreliableLink is the most basic link abstraction.
//
// It provides NO guarantees:
//   - Messages may be lost at any time
//   - Messages may be duplicated
//   - Messages may be reordered
//   - The loss rate is configurable
//
// This models a raw, unreliable network channel.
// It is NOT one of the formal abstractions in Cachin et al.,
// but serves as a teaching baseline to contrast with fair-loss links.
type UnreliableLink struct {
	// LossRate is the probability that any message is lost (0.0 to 1.0).
	LossRate float64
}

// NewUnreliableLink creates an unreliable link with the given loss rate.
func NewUnreliableLink(lossRate float64) *UnreliableLink {
	return &UnreliableLink{LossRate: lossRate}
}

// Send may drop the message based on LossRate.
func (l *UnreliableLink) Send(msg process.Message, rng *rand.Rand) []process.Message {
	if rng.Float64() < l.LossRate {
		return nil // Message lost
	}
	return []process.Message{msg}
}

// Tick does nothing for unreliable links (no retransmission).
func (l *UnreliableLink) Tick(step int, rng *rand.Rand) []process.Message {
	return nil
}

// Receive always accepts messages (no filtering).
func (l *UnreliableLink) Receive(msg process.Message) (process.Message, bool) {
	return msg, true
}
