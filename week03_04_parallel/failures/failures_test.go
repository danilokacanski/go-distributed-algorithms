package failures

import (
	"testing"

	"github.com/danilokacanski/da/week03_04_parallel/process"
)

// ============================================================================
// NO FAILURE TESTS
// ============================================================================

func TestNoFailure_AlwaysAlive(t *testing.T) {
	f := NewNoFailure()
	if !f.IsAlive("p1") {
		t.Fatal("NoFailure should always return alive")
	}
}

func TestNoFailure_AlwaysDelivers(t *testing.T) {
	f := NewNoFailure()
	msg := process.NewMessage("p", "q", "TEST", nil)
	if !f.ShouldDeliver(msg) {
		t.Fatal("NoFailure should always deliver")
	}
}

func TestNoFailure_NoAlter(t *testing.T) {
	f := NewNoFailure()
	msgs := []process.Message{process.NewMessage("p", "q", "TEST", nil)}
	result := f.MaybeAlter("p", msgs)
	if len(result) != 1 {
		t.Fatalf("NoFailure.MaybeAlter should return messages unchanged, got %d", len(result))
	}
}

// ============================================================================
// CRASH FAILURE TESTS
// ============================================================================

func TestCrashFailure_AliveByDefault(t *testing.T) {
	f := NewCrashFailure()
	if !f.IsAlive("p1") {
		t.Fatal("process should be alive by default")
	}
}

func TestCrashFailure_CrashProcess(t *testing.T) {
	f := NewCrashFailure()
	f.CrashProcess("p1")
	if f.IsAlive("p1") {
		t.Fatal("crashed process should not be alive")
	}
	if !f.IsAlive("p2") {
		t.Fatal("non-crashed process should still be alive")
	}
}

func TestCrashFailure_NoRecoveryInCrashStop(t *testing.T) {
	f := NewCrashFailure()
	f.CrashProcess("p1")
	f.RecoverProcess("p1")
	if f.IsAlive("p1") {
		t.Fatal("crash-stop model should not allow recovery")
	}
}

func TestCrashFailure_ConcurrentAccess(t *testing.T) {
	f := NewCrashFailure()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			f.CrashProcess(process.ProcessID("p"))
		}
		close(done)
	}()
	for i := 0; i < 100; i++ {
		f.IsAlive("p")
	}
	<-done
}

// ============================================================================
// OMISSION FAILURE TESTS
// ============================================================================

func TestOmissionFailure_NoOmissionByDefault(t *testing.T) {
	f := NewOmissionFailure(42)
	msg := process.NewMessage("p", "q", "TEST", nil)
	for i := 0; i < 100; i++ {
		if !f.ShouldDeliver(msg) {
			t.Fatal("should always deliver when no omission rate set")
		}
	}
}

func TestOmissionFailure_TotalOmission(t *testing.T) {
	f := NewOmissionFailure(42)
	f.SetOmissionRate("q", 1.0)
	msg := process.NewMessage("p", "q", "TEST", nil)
	for i := 0; i < 100; i++ {
		if f.ShouldDeliver(msg) {
			t.Fatal("should never deliver with 100% omission rate")
		}
	}
}

func TestOmissionFailure_PartialOmission(t *testing.T) {
	f := NewOmissionFailure(42)
	f.SetOmissionRate("q", 0.5)
	msg := process.NewMessage("p", "q", "TEST", nil)
	delivered := 0
	total := 1000
	for i := 0; i < total; i++ {
		if f.ShouldDeliver(msg) {
			delivered++
		}
	}
	if delivered < 350 || delivered > 650 {
		t.Fatalf("expected ~500 deliveries with 50%% omission, got %d", delivered)
	}
}

func TestOmissionFailure_AlwaysAlive(t *testing.T) {
	f := NewOmissionFailure(42)
	if !f.IsAlive("p1") {
		t.Fatal("omission processes should always be alive")
	}
}

// ============================================================================
// BYZANTINE FAILURE TESTS
// ============================================================================

func TestByzantineFailure_NoAlterIfNotByzantine(t *testing.T) {
	f := NewByzantineFailure()
	f.SetAlterFunc(func(_ process.ProcessID, msgs []process.Message) []process.Message {
		return append(msgs, process.NewMessage("attacker", "q", "EVIL", nil))
	})
	msgs := []process.Message{process.NewMessage("p", "q", "TEST", nil)}
	result := f.MaybeAlter("p", msgs)
	if len(result) != 1 {
		t.Fatalf("non-byzantine process should not be altered, got %d messages", len(result))
	}
}

func TestByzantineFailure_AltersIfByzantine(t *testing.T) {
	f := NewByzantineFailure()
	f.SetByzantine("p")
	f.SetAlterFunc(func(_ process.ProcessID, msgs []process.Message) []process.Message {
		return append(msgs, process.NewMessage("attacker", "q", "EVIL", nil))
	})
	msgs := []process.Message{process.NewMessage("p", "q", "TEST", nil)}
	result := f.MaybeAlter("p", msgs)
	if len(result) != 2 {
		t.Fatalf("byzantine process should have altered msgs, got %d messages", len(result))
	}
}

func TestByzantineFailure_NilAlterFunc(t *testing.T) {
	f := NewByzantineFailure()
	f.SetByzantine("p")
	msgs := []process.Message{process.NewMessage("p", "q", "TEST", nil)}
	result := f.MaybeAlter("p", msgs)
	if len(result) != 1 {
		t.Fatalf("nil alterFunc should return msgs unchanged, got %d", len(result))
	}
}

func TestByzantineFailure_ConcurrentAccess(t *testing.T) {
	f := NewByzantineFailure()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			f.SetByzantine("p")
		}
		close(done)
	}()
	msgs := []process.Message{process.NewMessage("p", "q", "TEST", nil)}
	for i := 0; i < 100; i++ {
		f.MaybeAlter("p", msgs)
	}
	<-done
}
