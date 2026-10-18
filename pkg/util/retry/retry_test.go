package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errTransient = errors.New("transient")
var errFatal = errors.New("fatal")

func TestDo_succeedsFirstAttempt(t *testing.T) {
	calls := 0
	val, err := Do(context.Background(), DoOptions{}, func(_ context.Context) (int, error) {
		calls++
		return 42, nil
	})
	if err != nil || val != 42 || calls != 1 {
		t.Fatalf("got val=%d err=%v calls=%d", val, err, calls)
	}
}

func TestDo_retriesAndSucceeds(t *testing.T) {
	calls := 0
	val, err := Do(context.Background(), DoOptions{MaxAttempts: 3, Base: time.Millisecond}, func(_ context.Context) (int, error) {
		calls++
		if calls < 3 {
			return 0, errTransient
		}
		return 7, nil
	})
	if err != nil || val != 7 || calls != 3 {
		t.Fatalf("got val=%d err=%v calls=%d", val, err, calls)
	}
}

func TestDo_exhaustsAttempts(t *testing.T) {
	calls := 0
	_, err := Do(context.Background(), DoOptions{MaxAttempts: 3, Base: time.Millisecond}, func(_ context.Context) (int, error) {
		calls++
		return 0, errTransient
	})
	if err == nil || calls != 3 {
		t.Fatalf("expected error after 3 attempts, got err=%v calls=%d", err, calls)
	}
}

func TestDo_nonRetryableStopsImmediately(t *testing.T) {
	calls := 0
	_, err := Do(context.Background(), DoOptions{
		MaxAttempts: 5,
		Base:        time.Millisecond,
		Retryable:   func(e error) bool { return !errors.Is(e, errFatal) },
	}, func(_ context.Context) (int, error) {
		calls++
		return 0, errFatal
	})
	if !errors.Is(err, errFatal) || calls != 1 {
		t.Fatalf("expected fatal after 1 call, got err=%v calls=%d", err, calls)
	}
}

func TestDo_ctxCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, err := Do(ctx, DoOptions{MaxAttempts: 3, Base: time.Millisecond}, func(_ context.Context) (int, error) {
		calls++
		return 0, errTransient
	})
	// First call runs, then ctx is checked before the sleep — should not hit attempt 2.
	if calls > 1 {
		t.Fatalf("expected at most 1 call with cancelled ctx, got %d", calls)
	}
	if err == nil {
		t.Fatal("expected non-nil error")
	}
}
