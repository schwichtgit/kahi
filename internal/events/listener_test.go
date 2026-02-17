package events

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewListenerPoolSubscribesAndDispatches(t *testing.T) {
	bus := NewBus(testLogger())
	lp := NewListenerPool("test-pool", bus, testLogger(), []EventType{ProcessStateRunning, ProcessStateStopped})

	// Should have subscribed to both event types.
	if bus.SubscriberCount(ProcessStateRunning) != 1 {
		t.Fatalf("expected 1 subscriber for ProcessStateRunning, got %d", bus.SubscriberCount(ProcessStateRunning))
	}
	if bus.SubscriberCount(ProcessStateStopped) != 1 {
		t.Fatalf("expected 1 subscriber for ProcessStateStopped, got %d", bus.SubscriberCount(ProcessStateStopped))
	}

	lp.Stop()
}

func TestAddListenerRegistersAcknowledged(t *testing.T) {
	bus := NewBus(testLogger())
	lp := NewListenerPool("test-pool", bus, testLogger(), []EventType{ProcessStateRunning})
	defer lp.Stop()

	// Create pipes. AddListener takes (stdin io.Writer, stdout io.Reader).
	// stdin: we write to stdinW, listener reads from stdinR -- pass stdinW as stdin arg.
	// stdout: listener writes to stdoutW, we read from stdoutR -- pass stdoutR as stdout arg.
	stdoutR, stdoutW := io.Pipe()
	_, stdinW := io.Pipe()

	lp.AddListener("listener-0", stdinW, stdoutR)

	lp.mu.Lock()
	if len(lp.listeners) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(lp.listeners))
	}
	l := lp.listeners[0]
	lp.mu.Unlock()

	l.mu.Lock()
	state := l.state
	l.mu.Unlock()

	if state != ListenerAcknowledged {
		t.Fatalf("expected state Acknowledged (0), got %d", state)
	}

	// Close pipes to let readReady goroutine exit.
	stdoutW.Close()
	stdinW.Close()
}

func TestReadReadyTransitionsToReady(t *testing.T) {
	bus := NewBus(testLogger())
	lp := NewListenerPool("test-pool", bus, testLogger(), []EventType{ProcessStateRunning})
	defer lp.Stop()

	stdoutR, stdoutW := io.Pipe()
	_, stdinW := io.Pipe()

	lp.AddListener("listener-0", stdinW, stdoutR)

	// Send READY token through the stdout pipe.
	fmt.Fprintln(stdoutW, "READY")

	// Wait for the readReady goroutine to process it.
	waitForState(t, lp, 0, ListenerReady, 2*time.Second)

	stdoutW.Close()
	stdinW.Close()
}

func TestDispatchRoutesEventsToSendToReady(t *testing.T) {
	bus := NewBus(testLogger())
	lp := NewListenerPool("test-pool", bus, testLogger(), []EventType{ProcessStateRunning})
	defer lp.Stop()

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()

	lp.AddListener("listener-0", stdinW, stdoutR)

	// Signal READY.
	fmt.Fprintln(stdoutW, "READY")

	// Wait for Ready state.
	waitForState(t, lp, 0, ListenerReady, 2*time.Second)

	// Publish an event through the bus.
	ts := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	bus.Publish(Event{
		Type:      ProcessStateRunning,
		Timestamp: ts,
		Data:      map[string]string{"name": "web"},
	})

	// Read the payload from the listener's stdin.
	scanner := bufio.NewScanner(stdinR)
	done := make(chan string, 1)
	go func() {
		if scanner.Scan() {
			done <- scanner.Text()
		}
	}()

	select {
	case line := <-done:
		if !strings.HasPrefix(line, "PROCESS_STATE_RUNNING") {
			t.Fatalf("expected payload starting with PROCESS_STATE_RUNNING, got %q", line)
		}
		if !strings.Contains(line, "name:web") {
			t.Fatalf("expected payload to contain name:web, got %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event payload on listener stdin")
	}

	stdoutW.Close()
	stdinR.Close()
}

func TestSendToReadyTransitionsToBusy(t *testing.T) {
	bus := NewBus(testLogger())
	lp := NewListenerPool("test-pool", bus, testLogger(), []EventType{ProcessStateRunning})
	defer lp.Stop()

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()

	lp.AddListener("listener-0", stdinW, stdoutR)

	// Signal READY.
	fmt.Fprintln(stdoutW, "READY")
	waitForState(t, lp, 0, ListenerReady, 2*time.Second)

	// Drain stdin in background so sendToReady doesn't block.
	go io.Copy(io.Discard, stdinR)

	// Send event directly.
	ts := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	lp.sendToReady(Event{
		Type:      ProcessStateRunning,
		Timestamp: ts,
		Data:      map[string]string{"pid": "123"},
	})

	// Listener should now be Busy.
	lp.mu.Lock()
	l := lp.listeners[0]
	lp.mu.Unlock()

	l.mu.Lock()
	state := l.state
	l.mu.Unlock()

	if state != ListenerBusy {
		t.Fatalf("expected state Busy (2), got %d", state)
	}

	stdoutW.Close()
	stdinR.Close()
}

func TestFormatEventPayload(t *testing.T) {
	ts := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	event := Event{
		Type:      ProcessStateRunning,
		Timestamp: ts,
		Data:      map[string]string{"name": "web"},
	}

	payload := formatEventPayload(event)

	// Payload format: "TYPE TIMESTAMP key:value\n"
	if !strings.HasPrefix(payload, "PROCESS_STATE_RUNNING ") {
		t.Fatalf("expected prefix PROCESS_STATE_RUNNING, got %q", payload)
	}
	if !strings.Contains(payload, ts.Format(time.RFC3339)) {
		t.Fatalf("expected RFC3339 timestamp in payload, got %q", payload)
	}
	if !strings.Contains(payload, "name:web") {
		t.Fatalf("expected name:web in payload, got %q", payload)
	}
	if !strings.HasSuffix(payload, "\n") {
		t.Fatalf("expected trailing newline, got %q", payload)
	}
}

func TestFormatEventPayloadEmptyData(t *testing.T) {
	ts := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	event := Event{
		Type:      ProcessStateStopped,
		Timestamp: ts,
		Data:      nil,
	}

	payload := formatEventPayload(event)
	expected := fmt.Sprintf("PROCESS_STATE_STOPPED %s\n", ts.Format(time.RFC3339))
	if payload != expected {
		t.Fatalf("expected %q, got %q", expected, payload)
	}
}

func TestMultipleListenersOnlyFirstReadyGetsEvent(t *testing.T) {
	bus := NewBus(testLogger())
	lp := NewListenerPool("test-pool", bus, testLogger(), []EventType{ProcessStateRunning})
	defer lp.Stop()

	// Set up two listeners.
	stdoutR0, stdoutW0 := io.Pipe()
	stdinR0, stdinW0 := io.Pipe()
	stdoutR1, stdoutW1 := io.Pipe()
	stdinR1, stdinW1 := io.Pipe()

	lp.AddListener("listener-0", stdinW0, stdoutR0)
	lp.AddListener("listener-1", stdinW1, stdoutR1)

	// Make both ready.
	fmt.Fprintln(stdoutW0, "READY")
	fmt.Fprintln(stdoutW1, "READY")
	waitForState(t, lp, 0, ListenerReady, 2*time.Second)
	waitForState(t, lp, 1, ListenerReady, 2*time.Second)

	// Read from both stdin pipes concurrently.
	var received0, received1 bool
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdinR0)
		if scanner.Scan() {
			mu.Lock()
			received0 = true
			mu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdinR1)
		if scanner.Scan() {
			mu.Lock()
			received1 = true
			mu.Unlock()
		}
	}()

	// Send one event.
	ts := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	lp.sendToReady(Event{
		Type:      ProcessStateRunning,
		Timestamp: ts,
		Data:      map[string]string{"name": "web"},
	})

	// Give time for delivery.
	time.Sleep(100 * time.Millisecond)

	// Close pipes to unblock scanners.
	stdinW0.Close()
	stdinW1.Close()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	// Only the first ready listener (listener-0) should have received the event.
	if !received0 {
		t.Fatal("expected listener-0 to receive the event")
	}
	if received1 {
		t.Fatal("listener-1 should not have received the event")
	}

	// listener-0 should be Busy, listener-1 should still be Ready.
	lp.mu.Lock()
	l0 := lp.listeners[0]
	l1 := lp.listeners[1]
	lp.mu.Unlock()

	l0.mu.Lock()
	state0 := l0.state
	l0.mu.Unlock()

	l1.mu.Lock()
	state1 := l1.state
	l1.mu.Unlock()

	if state0 != ListenerBusy {
		t.Fatalf("expected listener-0 to be Busy, got %d", state0)
	}
	if state1 != ListenerReady {
		t.Fatalf("expected listener-1 to still be Ready, got %d", state1)
	}

	stdoutW0.Close()
	stdoutW1.Close()
	stdinR0.Close()
	stdinR1.Close()
}

func TestNoReadyListenersDoesNotCrash(t *testing.T) {
	bus := NewBus(testLogger())
	lp := NewListenerPool("test-pool", bus, testLogger(), []EventType{ProcessStateRunning})
	defer lp.Stop()

	// Add a listener but never send READY.
	stdoutR, stdoutW := io.Pipe()
	_, stdinW := io.Pipe()
	lp.AddListener("listener-0", stdinW, stdoutR)

	// Send event with no ready listeners -- should not panic or block.
	ts := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	lp.sendToReady(Event{
		Type:      ProcessStateRunning,
		Timestamp: ts,
	})

	// Also test with zero listeners at all.
	bus2 := NewBus(testLogger())
	lp2 := NewListenerPool("empty-pool", bus2, testLogger(), []EventType{ProcessStateRunning})
	defer lp2.Stop()

	lp2.sendToReady(Event{
		Type:      ProcessStateRunning,
		Timestamp: ts,
	})

	stdoutW.Close()
	stdinW.Close()
}

func TestStopShutsDownCleanly(t *testing.T) {
	bus := NewBus(testLogger())
	lp := NewListenerPool("test-pool", bus, testLogger(), []EventType{ProcessStateRunning, ProcessStateStopped})

	// Verify subscriptions exist before stop.
	if bus.SubscriberCount(ProcessStateRunning) != 1 {
		t.Fatalf("expected 1 subscriber before stop, got %d", bus.SubscriberCount(ProcessStateRunning))
	}

	lp.Stop()

	// After stop, subscriptions should be removed.
	if bus.SubscriberCount(ProcessStateRunning) != 0 {
		t.Fatalf("expected 0 subscribers after stop, got %d", bus.SubscriberCount(ProcessStateRunning))
	}
	if bus.SubscriberCount(ProcessStateStopped) != 0 {
		t.Fatalf("expected 0 subscribers after stop, got %d", bus.SubscriberCount(ProcessStateStopped))
	}

	// done channel should be closed (non-blocking read).
	select {
	case <-lp.done:
		// Expected.
	default:
		t.Fatal("done channel not closed after Stop")
	}
}

func TestEventQueueFullDoesNotBlockPublisher(t *testing.T) {
	bus := NewBus(testLogger())

	// Create pool but do not start any listeners or drain eventCh.
	lp := &ListenerPool{
		name:    "full-pool",
		bus:     bus,
		logger:  testLogger(),
		eventCh: make(chan Event, 2), // Small buffer for testing.
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}

	// Subscribe manually to route events into the small channel.
	id := bus.Subscribe(ProcessStateRunning, func(e Event) {
		select {
		case lp.eventCh <- e:
		default:
			// Queue full; drop event. This matches NewListenerPool behavior.
		}
	})
	lp.subIDs = append(lp.subIDs, id)

	// Fill the queue.
	ts := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	bus.Publish(Event{Type: ProcessStateRunning, Timestamp: ts})
	bus.Publish(Event{Type: ProcessStateRunning, Timestamp: ts})

	// This publish should not block even though the queue is full.
	done := make(chan struct{})
	go func() {
		bus.Publish(Event{Type: ProcessStateRunning, Timestamp: ts})
		close(done)
	}()

	select {
	case <-done:
		// Publisher returned without blocking.
	case <-time.After(1 * time.Second):
		t.Fatal("publisher blocked on full event queue")
	}

	// Cleanup.
	bus.Unsubscribe(id)
	close(lp.stopCh)
	close(lp.done)
}

// waitForState polls until the listener at the given index reaches the expected state,
// or fails the test after the timeout.
func waitForState(t *testing.T, lp *ListenerPool, index int, expected ListenerState, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)
	for {
		lp.mu.Lock()
		if index >= len(lp.listeners) {
			lp.mu.Unlock()
			t.Fatalf("listener index %d out of range", index)
		}
		l := lp.listeners[index]
		lp.mu.Unlock()

		l.mu.Lock()
		state := l.state
		l.mu.Unlock()

		if state == expected {
			return
		}

		select {
		case <-deadline:
			t.Fatalf("timed out waiting for listener %d to reach state %d (current: %d)", index, expected, state)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
