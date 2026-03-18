package link

import (
	"testing"

	"github.com/danilokacanski/da/week0203_parallel/crypto"
	"github.com/danilokacanski/da/week0203_parallel/process"
)

// ============================================================================
// FAIR-LOSS LINK TESTS
// ============================================================================

func TestFairLossLink_NoLoss(t *testing.T) {
	fll := NewFairLossLink(0.0, 1)
	msg := process.NewMessage("p", "q", "TEST", "hello")
	for i := 0; i < 100; i++ {
		result := fll.Send(msg)
		if len(result) != 1 {
			t.Fatalf("expected 1 message with 0%% loss, got %d", len(result))
		}
	}
}

func TestFairLossLink_TotalLoss(t *testing.T) {
	fll := NewFairLossLink(1.0, 1)
	msg := process.NewMessage("p", "q", "TEST", "hello")
	for i := 0; i < 100; i++ {
		result := fll.Send(msg)
		if len(result) != 0 {
			t.Fatalf("expected 0 messages with 100%% loss, got %d", len(result))
		}
	}
}

func TestFairLossLink_PartialLoss(t *testing.T) {
	fll := NewFairLossLink(0.5, 42)
	msg := process.NewMessage("p", "q", "TEST", "hello")
	delivered := 0
	total := 1000
	for i := 0; i < total; i++ {
		result := fll.Send(msg)
		delivered += len(result)
	}
	if delivered < 350 || delivered > 650 {
		t.Fatalf("expected ~500 deliveries, got %d (out of %d)", delivered, total)
	}
}

func TestFairLossLink_ReceiveAlwaysAccepts(t *testing.T) {
	fll := NewFairLossLink(0.5, 1)
	msg := process.NewMessage("p", "q", "TEST", "hello")
	_, ok := fll.Receive(msg)
	if !ok {
		t.Fatal("FairLossLink.Receive should always accept")
	}
}

func TestFairLossLink_NoRetransmissions(t *testing.T) {
	fll := NewFairLossLink(0.0, 1)
	if result := fll.Retransmissions(); result != nil {
		t.Fatal("FairLossLink.Retransmissions should return nil")
	}
}

// ============================================================================
// STUBBORN LINK TESTS
// ============================================================================

func TestStubbornLink_StoresMessages(t *testing.T) {
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	msg1 := process.NewMessage("p", "q", "TEST", "hello")
	msg2 := process.NewMessage("p", "q", "TEST", "world")
	sl.Send(msg1)
	sl.Send(msg2)
	retransmissions := sl.Retransmissions()
	if len(retransmissions) != 2 {
		t.Fatalf("expected 2 retransmissions, got %d", len(retransmissions))
	}
}

func TestStubbornLink_RetransmitsWithLoss(t *testing.T) {
	fll := NewFairLossLink(0.5, 42)
	sl := NewStubbornLink(fll)
	msg := process.NewMessage("p", "q", "TEST", "hello")
	sl.Send(msg)
	totalDelivered := 0
	for i := 0; i < 100; i++ {
		retransmissions := sl.Retransmissions()
		totalDelivered += len(retransmissions)
	}
	if totalDelivered == 0 {
		t.Fatal("expected at least some retransmissions to get through with 50% loss")
	}
}

func TestStubbornLink_ReceiveAlwaysAccepts(t *testing.T) {
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	msg := process.NewMessage("p", "q", "TEST", "hello")
	_, ok := sl.Receive(msg)
	if !ok {
		t.Fatal("StubbornLink.Receive should always accept (dedup is not its job)")
	}
}

// ============================================================================
// PERFECT LINK TESTS
// ============================================================================

func TestPerfectLink_Deduplication(t *testing.T) {
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	pl := NewPerfectLink(sl)
	msg := process.NewMessage("p", "q", "TEST", "hello")
	_, ok := pl.Receive(msg)
	if !ok {
		t.Fatal("first Receive should accept the message")
	}
	_, ok = pl.Receive(msg)
	if ok {
		t.Fatal("second Receive should reject the duplicate")
	}
}

func TestPerfectLink_DifferentMessagesAllDelivered(t *testing.T) {
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	pl := NewPerfectLink(sl)
	for i := 0; i < 10; i++ {
		msg := process.NewMessage("p", "q", "TEST", i)
		_, ok := pl.Receive(msg)
		if !ok {
			t.Fatalf("message %d should be accepted (first occurrence)", i)
		}
	}
}

func TestPerfectLink_SendGoesThrough(t *testing.T) {
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	pl := NewPerfectLink(sl)
	msg := process.NewMessage("p", "q", "TEST", "hello")
	result := pl.Send(msg)
	if len(result) != 1 {
		t.Fatalf("expected 1 message with 0%% loss, got %d", len(result))
	}
}

// ============================================================================
// AUTHENTICATED LINK TESTS
// ============================================================================

func TestAuthenticatedLink_ValidMAC(t *testing.T) {
	keys := crypto.NewKeyRegistry()
	keys.RegisterProcess("alice")
	keys.RegisterProcess("bob")
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	pl := NewPerfectLink(sl)
	al := NewAuthenticatedLink(pl, keys)
	msg := process.NewMessage("alice", "bob", "SECRET", "hello")
	sent := al.Send(msg)
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	_, ok := al.Receive(sent[0])
	if !ok {
		t.Fatal("authenticated message should be accepted")
	}
}

func TestAuthenticatedLink_TamperedRejected(t *testing.T) {
	keys := crypto.NewKeyRegistry()
	keys.RegisterProcess("alice")
	keys.RegisterProcess("bob")
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	pl := NewPerfectLink(sl)
	al := NewAuthenticatedLink(pl, keys)
	msg := process.NewMessage("alice", "bob", "SECRET", "hello")
	sent := al.Send(msg)
	tampered := sent[0].Clone()
	tampered.Data = "TAMPERED"
	_, ok := al.Receive(tampered)
	if ok {
		t.Fatal("tampered message should be rejected")
	}
}

func TestAuthenticatedLink_NoMAC_Rejected(t *testing.T) {
	keys := crypto.NewKeyRegistry()
	keys.RegisterProcess("alice")
	keys.RegisterProcess("bob")
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	pl := NewPerfectLink(sl)
	al := NewAuthenticatedLink(pl, keys)
	msg := process.NewMessage("alice", "bob", "SECRET", "hello")
	_, ok := al.Receive(msg)
	if ok {
		t.Fatal("message without MAC should be rejected")
	}
}

func TestAuthenticatedLink_TamperedDoesNotBurnDedupSlot(t *testing.T) {
	keys := crypto.NewKeyRegistry()
	keys.RegisterProcess("alice")
	keys.RegisterProcess("bob")
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	pl := NewPerfectLink(sl)
	al := NewAuthenticatedLink(pl, keys)
	msg := process.NewMessage("alice", "bob", "SECRET", "hello")
	sent := al.Send(msg)
	tampered := sent[0].Clone()
	tampered.Meta["mac"] = "corrupted_mac_value"
	_, ok := al.Receive(tampered)
	if ok {
		t.Fatal("tampered message should be rejected")
	}
	_, ok = al.Receive(sent[0])
	if !ok {
		t.Fatal("original valid message should still be accepted after tampered rejection — dedup slot was burned!")
	}
}

func TestAuthenticatedLink_DuplicateAfterValid_Rejected(t *testing.T) {
	keys := crypto.NewKeyRegistry()
	keys.RegisterProcess("alice")
	keys.RegisterProcess("bob")
	fll := NewFairLossLink(0.0, 1)
	sl := NewStubbornLink(fll)
	pl := NewPerfectLink(sl)
	al := NewAuthenticatedLink(pl, keys)
	msg := process.NewMessage("alice", "bob", "SECRET", "hello")
	sent := al.Send(msg)
	_, ok := al.Receive(sent[0])
	if !ok {
		t.Fatal("first Receive should accept")
	}
	_, ok = al.Receive(sent[0])
	if ok {
		t.Fatal("duplicate should be rejected by dedup")
	}
}
