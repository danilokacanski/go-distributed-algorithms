package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danilokacanski/da/week0203_parallel/failures"
	"github.com/danilokacanski/da/week0203_parallel/link"
	"github.com/danilokacanski/da/week0203_parallel/process"
)

// ============================================================================
// TRACE TESTS
// ============================================================================

func TestTrace_SeqOrdering(t *testing.T) {
	tr := NewTrace(false)

	done := make(chan struct{})
	n := 100
	for i := 0; i < n; i++ {
		go func() {
			tr.Log(Event{Type: EventSend, Detail: "test"})
			done <- struct{}{}
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}

	events := tr.Events()
	if len(events) != n {
		t.Fatalf("expected %d events, got %d", n, len(events))
	}
	for i := 1; i < len(events); i++ {
		if events[i].Seq <= events[i-1].Seq {
			t.Fatalf("events not in order: seq %d at index %d, seq %d at index %d",
				events[i-1].Seq, i-1, events[i].Seq, i)
		}
	}
}

func TestTrace_EventsSnapshot(t *testing.T) {
	tr := NewTrace(false)
	tr.Log(Event{Type: EventSend, Detail: "msg1"})

	snap1 := tr.Events()
	tr.Log(Event{Type: EventSend, Detail: "msg2"})
	snap2 := tr.Events()

	if len(snap1) != 1 {
		t.Fatalf("snap1 should have 1 event, got %d", len(snap1))
	}
	if len(snap2) != 2 {
		t.Fatalf("snap2 should have 2 events, got %d", len(snap2))
	}
}

func TestTrace_EventsOfType(t *testing.T) {
	tr := NewTrace(false)
	tr.Log(Event{Type: EventSend, Detail: "send"})
	tr.Log(Event{Type: EventDeliver, Detail: "deliver"})
	tr.Log(Event{Type: EventSend, Detail: "send2"})

	sends := tr.EventsOfType(EventSend)
	if len(sends) != 2 {
		t.Fatalf("expected 2 SEND events, got %d", len(sends))
	}
}

// ============================================================================
// CHECKER TESTS
// ============================================================================

func TestNoCreationChecker_Pass(t *testing.T) {
	msg := process.NewMessage("p", "q", "TEST", "hello")
	events := []Event{
		{Type: EventSend, Message: &msg},
		{Type: EventDeliver, Message: &msg},
	}
	checker := NoCreationChecker()
	if v := checker.Check(events); v != "" {
		t.Fatalf("should pass: %s", v)
	}
}

func TestNoCreationChecker_Fail(t *testing.T) {
	msg := process.NewMessage("p", "q", "TEST", "hello")
	events := []Event{
		{Type: EventDeliver, Message: &msg},
	}
	checker := NoCreationChecker()
	if v := checker.Check(events); v == "" {
		t.Fatal("should detect no-creation violation")
	}
}

func TestNoDuplicationChecker_Pass(t *testing.T) {
	msg := process.NewMessage("p", "q", "TEST", "hello")
	events := []Event{
		{Type: EventDeliver, Message: &msg},
	}
	checker := NoDuplicationChecker()
	if v := checker.Check(events); v != "" {
		t.Fatalf("should pass: %s", v)
	}
}

func TestNoDuplicationChecker_Fail(t *testing.T) {
	msg := process.NewMessage("p", "q", "TEST", "hello")
	events := []Event{
		{Type: EventDeliver, Message: &msg},
		{Type: EventDeliver, Message: &msg},
	}
	checker := NoDuplicationChecker()
	if v := checker.Check(events); v == "" {
		t.Fatal("should detect duplication violation")
	}
}

func TestReliableDeliveryChecker_Pass(t *testing.T) {
	msg := process.NewMessage("p", "q", "TEST", "hello")
	events := []Event{
		{Type: EventSend, Message: &msg},
		{Type: EventDeliver, Message: &msg},
	}
	checker := ReliableDeliveryChecker()
	if v := checker.Check(events); v != "" {
		t.Fatalf("should pass: %s", v)
	}
}

func TestReliableDeliveryChecker_Fail(t *testing.T) {
	msg := process.NewMessage("p", "q", "TEST", "hello")
	events := []Event{
		{Type: EventSend, Message: &msg},
	}
	checker := ReliableDeliveryChecker()
	if v := checker.Check(events); v == "" {
		t.Fatal("should detect reliable-delivery violation")
	}
}

// ============================================================================
// RUNTIME INTEGRATION TESTS
// ============================================================================

type echoProcess struct {
	id    process.ProcessID
	Count int64
}

func (p *echoProcess) ID() process.ProcessID { return p.id }

func (p *echoProcess) Run(ctx context.Context, inbox <-chan process.Message, send func(process.Message)) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-inbox:
			if !ok {
				return
			}
			if msg.Type == "PING" {
				atomic.AddInt64(&p.Count, 1)
				send(process.NewMessage(p.id, msg.From, "PONG", msg.Data))
			}
		}
	}
}

type sinkProcess struct {
	id    process.ProcessID
	Count int64
}

func (p *sinkProcess) ID() process.ProcessID { return p.id }

func (p *sinkProcess) Run(ctx context.Context, inbox <-chan process.Message, send func(process.Message)) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-inbox:
			if !ok {
				return
			}
			if msg.Type == "DATA" {
				atomic.AddInt64(&p.Count, 1)
			}
		}
	}
}

type testSender struct {
	id     process.ProcessID
	target process.ProcessID
	count  int
}

func (s *testSender) ID() process.ProcessID { return s.id }
func (s *testSender) Handle(msg process.Message) []process.Message {
	if msg.Type != "INIT" {
		return nil
	}
	msgs := make([]process.Message, s.count)
	for i := 0; i < s.count; i++ {
		msgs[i] = process.NewMessage(s.id, s.target, "DATA", i)
	}
	return msgs
}

type countHandler struct {
	id    process.ProcessID
	count int
}

func (h *countHandler) ID() process.ProcessID { return h.id }
func (h *countHandler) Handle(msg process.Message) []process.Message {
	if msg.Type == "DATA" {
		h.count++
	}
	return nil
}

type dataSink struct {
	id       process.ProcessID
	lastData string
}

func (p *dataSink) ID() process.ProcessID { return p.id }
func (p *dataSink) Run(ctx context.Context, inbox <-chan process.Message, send func(process.Message)) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-inbox:
			if !ok {
				return
			}
			if msg.Type == "DATA" {
				if d, ok := msg.Data.(string); ok {
					p.lastData = d
				}
			}
		}
	}
}

func TestRuntime_BasicPingPong(t *testing.T) {
	fll := link.NewFairLossLink(0.0, 42)
	sl := link.NewStubbornLink(fll)
	pl := link.NewPerfectLink(sl)
	fm := failures.NewNoFailure()

	rt := NewRuntime(pl, fm,
		WithIdleTimeout(200*time.Millisecond),
		WithMaxDuration(3*time.Second),
		WithVerbose(false),
	)

	echo := &echoProcess{id: "echo"}
	sink := &sinkProcess{id: "sender"}
	rt.Register(echo)
	rt.Register(sink)

	rt.Inject(process.NewMessage("system", "echo", "PING", "test"))
	rt.Run()

	if atomic.LoadInt64(&echo.Count) != 1 {
		t.Fatalf("echo should have received 1 PING, got %d", echo.Count)
	}
}

func TestRuntime_PerfectLinkDelivers(t *testing.T) {
	fll := link.NewFairLossLink(0.5, 42)
	sl := link.NewStubbornLink(fll)
	pl := link.NewPerfectLink(sl)
	fm := failures.NewNoFailure()

	rt := NewRuntime(pl, fm,
		WithIdleTimeout(500*time.Millisecond),
		WithRetransmitInterval(50*time.Millisecond),
		WithMaxDuration(5*time.Second),
		WithVerbose(false),
	)

	sink := &sinkProcess{id: "sink"}
	rt.Register(sink)

	sender := &testSender{id: "sender", target: "sink", count: 5}
	rt.Register(process.WrapHandler(sender))

	rt.Inject(process.NewMessage("system", "sender", "INIT", nil))
	rt.Run()

	received := atomic.LoadInt64(&sink.Count)
	if received != 5 {
		t.Fatalf("perfect link should deliver all 5 messages, got %d", received)
	}
}

func TestRuntime_CrashStopDropsMessages(t *testing.T) {
	fll := link.NewFairLossLink(0.0, 42)
	sl := link.NewStubbornLink(fll)
	pl := link.NewPerfectLink(sl)
	fm := failures.NewCrashFailure()

	rt := NewRuntime(pl, fm,
		WithIdleTimeout(300*time.Millisecond),
		WithMaxDuration(3*time.Second),
		WithVerbose(false),
	)

	sink := &sinkProcess{id: "sink"}
	rt.Register(sink)
	rt.Register(process.WrapHandler(&testSender{id: "sender", target: "sink", count: 3}))

	rt.CrashProcess("sink")

	rt.Inject(process.NewMessage("system", "sender", "INIT", nil))
	rt.Run()

	received := atomic.LoadInt64(&sink.Count)
	if received != 0 {
		t.Fatalf("crashed process should receive 0 messages, got %d", received)
	}
}

func TestRuntime_FairLossLosesMessages(t *testing.T) {
	fll := link.NewFairLossLink(0.5, 42)
	fm := failures.NewNoFailure()

	rt := NewRuntime(fll, fm,
		WithIdleTimeout(300*time.Millisecond),
		WithMaxDuration(3*time.Second),
		WithVerbose(false),
	)

	sink := &sinkProcess{id: "sink"}
	rt.Register(sink)
	rt.Register(process.WrapHandler(&testSender{id: "sender", target: "sink", count: 20}))

	rt.Inject(process.NewMessage("system", "sender", "INIT", nil))
	rt.Run()

	received := atomic.LoadInt64(&sink.Count)
	if received == 20 {
		t.Log("warning: all 20 messages delivered despite 50% loss (possible but unlikely)")
	}
	if received == 0 {
		t.Fatal("expected at least some messages to be delivered with 50% loss")
	}
	t.Logf("received %d out of 20 messages (50%% loss link)", received)
}

func TestRuntime_IdleTimeout(t *testing.T) {
	fll := link.NewFairLossLink(0.0, 42)
	fm := failures.NewNoFailure()

	start := time.Now()
	rt := NewRuntime(fll, fm,
		WithIdleTimeout(200*time.Millisecond),
		WithMaxDuration(10*time.Second),
		WithVerbose(false),
	)

	sink := &sinkProcess{id: "sink"}
	rt.Register(sink)

	rt.Inject(process.NewMessage("system", "sink", "DATA", "hello"))
	rt.Run()

	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Fatalf("expected idle timeout to kick in quickly, but took %s", elapsed)
	}
}

func TestRuntime_MaxDuration(t *testing.T) {
	fll := link.NewFairLossLink(0.0, 42)
	sl := link.NewStubbornLink(fll)
	pl := link.NewPerfectLink(sl)
	fm := failures.NewNoFailure()

	start := time.Now()
	rt := NewRuntime(pl, fm,
		WithIdleTimeout(10*time.Second),
		WithMaxDuration(500*time.Millisecond),
		WithVerbose(false),
	)

	echo1 := &echoProcess{id: "a"}
	echo2 := &echoProcess{id: "b"}
	rt.Register(echo1)
	rt.Register(echo2)

	rt.Inject(process.NewMessage("system", "a", "PING", "start"))
	rt.Run()

	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		t.Fatalf("max duration should have stopped simulation, but took %s", elapsed)
	}
}

func TestRuntime_WrapHandler(t *testing.T) {
	fll := link.NewFairLossLink(0.0, 42)
	fm := failures.NewNoFailure()

	rt := NewRuntime(fll, fm,
		WithIdleTimeout(300*time.Millisecond),
		WithMaxDuration(3*time.Second),
		WithVerbose(false),
	)

	counter := &countHandler{id: "counter"}
	rt.Register(process.WrapHandler(counter))
	rt.Register(process.WrapHandler(&testSender{id: "sender", target: "counter", count: 3}))

	rt.Inject(process.NewMessage("system", "sender", "INIT", nil))
	rt.Run()

	if counter.count != 3 {
		t.Fatalf("counter should have received 3 DATA messages, got %d", counter.count)
	}
}

func TestRuntime_InterceptorModifiesMessages(t *testing.T) {
	fll := link.NewFairLossLink(0.0, 42)
	fm := failures.NewNoFailure()

	rt := NewRuntime(fll, fm,
		WithIdleTimeout(300*time.Millisecond),
		WithMaxDuration(3*time.Second),
		WithVerbose(false),
	)

	sink := &dataSink{id: "sink"}
	rt.Register(sink)
	rt.Register(process.WrapHandler(&testSender{id: "sender", target: "sink", count: 1}))

	rt.SetInterceptor(func(msg process.Message) process.Message {
		if msg.Type == "DATA" {
			m := msg.Clone()
			m.Data = "modified"
			return m
		}
		return msg
	})

	rt.Inject(process.NewMessage("system", "sender", "INIT", nil))
	rt.Run()

	if sink.lastData != "modified" {
		t.Fatalf("interceptor should have modified data, got %q", sink.lastData)
	}
}

func TestRuntime_NoDuplicationWithPerfectLink(t *testing.T) {
	fll := link.NewFairLossLink(0.3, 42)
	sl := link.NewStubbornLink(fll)
	pl := link.NewPerfectLink(sl)
	fm := failures.NewNoFailure()

	rt := NewRuntime(pl, fm,
		WithIdleTimeout(400*time.Millisecond),
		WithRetransmitInterval(50*time.Millisecond),
		WithMaxDuration(5*time.Second),
		WithVerbose(false),
	)

	rt.AddChecker(NoDuplicationChecker())

	sink := &sinkProcess{id: "sink"}
	rt.Register(sink)
	rt.Register(process.WrapHandler(&testSender{id: "sender", target: "sink", count: 5}))

	rt.Inject(process.NewMessage("system", "sender", "INIT", nil))
	rt.Run()

	for _, e := range rt.Trace().Events() {
		if e.Type == EventViolation {
			t.Fatalf("unexpected violation: %s", e.Detail)
		}
	}

	received := atomic.LoadInt64(&sink.Count)
	if received != 5 {
		t.Fatalf("perfect link should deliver all 5 messages exactly once, got %d", received)
	}
}
