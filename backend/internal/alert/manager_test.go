package alert

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type recordingNotifier struct {
	mu   sync.Mutex
	sent []Alert
	err  error
}

func (n *recordingNotifier) Name() string { return "recording" }

func (n *recordingNotifier) Send(_ context.Context, a Alert) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, a)
	return n.err
}

func (n *recordingNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.sent)
}

func newTestManager(cooldown time.Duration) (*Manager, *recordingNotifier, *time.Time) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	m := NewManager(cooldown, slog.New(slog.DiscardHandler))
	m.now = func() time.Time { return now }

	n := &recordingNotifier{}
	m.Register(n)
	return m, n, &now
}

func TestDispatchDeliversAndRecords(t *testing.T) {
	m, n, _ := newTestManager(time.Minute)

	fired := m.Dispatch(context.Background(), Alert{Key: "k1", RuleID: "r", Title: "t"})
	if !fired {
		t.Fatal("Dispatch() = false, want true for a fresh key")
	}
	if n.count() != 1 {
		t.Errorf("delivered = %d, want 1", n.count())
	}

	recent := m.Recent()
	if len(recent) != 1 || recent[0].Key != "k1" {
		t.Errorf("Recent() = %+v", recent)
	}
	if recent[0].FiredAt.IsZero() {
		t.Error("FiredAt must be stamped by the manager")
	}
}

func TestDispatchSuppressesRepeatsWithinCooldown(t *testing.T) {
	m, n, now := newTestManager(10 * time.Minute)
	a := Alert{Key: "k1", RuleID: "r", Title: "t"}

	m.Dispatch(context.Background(), a)
	if m.Dispatch(context.Background(), a) {
		t.Error("repeat within cooldown must be suppressed")
	}
	if n.count() != 1 {
		t.Errorf("delivered = %d, want 1 after suppression", n.count())
	}

	// A different key is a different condition and always fires.
	if !m.Dispatch(context.Background(), Alert{Key: "k2", RuleID: "r", Title: "t"}) {
		t.Error("different key must not be suppressed")
	}

	// After the window passes, the same key fires again.
	*now = now.Add(11 * time.Minute)
	if !m.Dispatch(context.Background(), a) {
		t.Error("repeat after cooldown must fire")
	}
	if n.count() != 3 {
		t.Errorf("delivered = %d, want 3", n.count())
	}
}

func TestDispatchSurvivesNotifierFailure(t *testing.T) {
	m, n, _ := newTestManager(time.Minute)
	n.err = errors.New("webhook 500")

	healthy := &recordingNotifier{}
	m.Register(healthy)

	if !m.Dispatch(context.Background(), Alert{Key: "k", RuleID: "r"}) {
		t.Fatal("Dispatch() must succeed even when a notifier fails")
	}
	if healthy.count() != 1 {
		t.Error("a broken notifier must not block the remaining channels")
	}
}

func TestRecentIsBoundedAndNewestFirst(t *testing.T) {
	m, _, _ := newTestManager(0)

	for i := 0; i < recentCapacity+20; i++ {
		m.Dispatch(context.Background(), Alert{Key: keyN(i), RuleID: "r"})
	}

	recent := m.Recent()
	if len(recent) != recentCapacity {
		t.Fatalf("Recent() length = %d, want %d", len(recent), recentCapacity)
	}
	if recent[0].Key != keyN(recentCapacity+19) {
		t.Errorf("Recent()[0].Key = %q, want newest", recent[0].Key)
	}
}

func TestTestBypassesCooldownAndReportsPerNotifier(t *testing.T) {
	m, n, _ := newTestManager(time.Hour)

	// Both notifiers share the Name() "recording", so give the broken one its
	// own manager slot via a distinct wrapper name.
	broken := namedNotifier{name: "broken", inner: &recordingNotifier{err: errors.New("boom")}}
	m.Register(broken)

	_, results := m.Test(context.Background())
	// A repeat must not be suppressed: cooldown does not apply to test fires.
	_, results2 := m.Test(context.Background())

	if results["recording"] != "ok" || results2["recording"] != "ok" {
		t.Errorf("healthy notifier results = %v / %v, want ok", results, results2)
	}
	if results["broken"] != "boom" {
		t.Errorf("broken notifier result = %q, want the delivery error", results["broken"])
	}
	if n.count() != 2 {
		t.Errorf("delivered = %d, want 2", n.count())
	}
}

func TestSetWebhookInstallsReplacesAndRemoves(t *testing.T) {
	m, _, _ := newTestManager(0)

	first := &recordingNotifier{}
	m.SetWebhook(namedNotifier{name: "webhook-1", inner: first})
	m.Dispatch(context.Background(), Alert{Key: "a", RuleID: "r"})
	if first.count() != 1 {
		t.Errorf("installed webhook deliveries = %d, want 1", first.count())
	}

	second := &recordingNotifier{}
	m.SetWebhook(namedNotifier{name: "webhook-2", inner: second})
	m.Dispatch(context.Background(), Alert{Key: "b", RuleID: "r"})
	if first.count() != 1 || second.count() != 1 {
		t.Errorf("after replace: first=%d second=%d, want 1/1", first.count(), second.count())
	}

	m.SetWebhook(nil)
	m.Dispatch(context.Background(), Alert{Key: "c", RuleID: "r"})
	if second.count() != 1 {
		t.Errorf("removed webhook still received deliveries: %d", second.count())
	}
}

func TestSetCooldownAppliesToNextDispatch(t *testing.T) {
	m, n, now := newTestManager(time.Hour)
	a := Alert{Key: "k", RuleID: "r"}

	m.Dispatch(context.Background(), a)
	m.SetCooldown(time.Minute)
	*now = now.Add(2 * time.Minute)

	if !m.Dispatch(context.Background(), a) {
		t.Error("shortened cooldown must let the repeat fire")
	}
	if n.count() != 2 {
		t.Errorf("deliveries = %d, want 2", n.count())
	}
}

type namedNotifier struct {
	name  string
	inner Notifier
}

func (n namedNotifier) Name() string                            { return n.name }
func (n namedNotifier) Send(ctx context.Context, a Alert) error { return n.inner.Send(ctx, a) }

func keyN(i int) string {
	return "key-" + string(rune('A'+i%26)) + "-" + time.Duration(i).String()
}
