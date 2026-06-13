package syncloop

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunStopsWhenStopChannelCloses(t *testing.T) {
	stop := make(chan struct{})
	var calls atomic.Int64

	Run(stop, 5*time.Millisecond, "test", func() error {
		calls.Add(1)
		return nil
	})

	// Let a few ticks happen, then stop.
	time.Sleep(30 * time.Millisecond)
	close(stop)
	time.Sleep(20 * time.Millisecond)
	after := calls.Load()
	if after == 0 {
		t.Fatal("sync function never ran")
	}

	// No further calls after stop.
	time.Sleep(40 * time.Millisecond)
	if calls.Load() != after {
		t.Fatalf("sync kept running after stop: %d -> %d", after, calls.Load())
	}
}

func TestRunBacksOffOnConsecutiveFailures(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	var calls atomic.Int64

	Run(stop, 10*time.Millisecond, "test", func() error {
		calls.Add(1)
		return errors.New("API returned status 429")
	})

	// With a 10ms base interval and exponential backoff (10, 20, 40, 80...),
	// 100ms allows at most ~4 failing attempts. Without backoff there would
	// be ~10.
	time.Sleep(100 * time.Millisecond)
	got := calls.Load()
	if got == 0 {
		t.Fatal("sync function never ran")
	}
	if got > 5 {
		t.Fatalf("expected backoff to throttle failing sync, got %d calls in 100ms", got)
	}
}

func TestRunRecoversIntervalAfterSuccess(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	var calls atomic.Int64

	// Fail twice, then succeed forever.
	Run(stop, 5*time.Millisecond, "test", func() error {
		n := calls.Add(1)
		if n <= 2 {
			return errors.New("transient")
		}
		return nil
	})

	// Failures at 5ms and 15ms (backoff 10ms), success at ~35ms (backoff 20ms),
	// then the interval resets to 5ms — plenty of successful runs by 150ms.
	time.Sleep(150 * time.Millisecond)
	if got := calls.Load(); got < 8 {
		t.Fatalf("expected interval to reset after success, got only %d calls", got)
	}
}
