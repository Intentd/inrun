package retry

import (
	"errors"
	"testing"
	"time"
)

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := Retry(func() error {
		calls++
		return nil
	}, Options{Attempts: 3, Delay: 0})
	if err != nil || calls != 1 {
		t.Errorf("expected 1 call, got %d; err=%v", calls, err)
	}
}

func TestRetry_SuccessOnRetry(t *testing.T) {
	calls := 0
	err := Retry(func() error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	}, Options{Attempts: 5, Delay: 0})
	if err != nil {
		t.Errorf("expected nil after retry, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_AllAttemptsFail(t *testing.T) {
	sentinel := errors.New("always fails")
	err := Retry(func() error {
		return sentinel
	}, Options{Attempts: 3, Delay: 0})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestRetry_ZeroAttempts(t *testing.T) {
	err := Retry(func() error { return nil }, Options{Attempts: 0, Delay: 0})
	if err == nil {
		t.Error("Retry with 0 attempts must return an error")
	}
}

func TestRetry_OneAttempt_Failure(t *testing.T) {
	sentinel := errors.New("fail")
	err := Retry(func() error { return sentinel }, Options{Attempts: 1, Delay: 0})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
}

func TestRetryBackoff_SuccessOnRetry(t *testing.T) {
	calls := 0
	err := RetryBackoff(func() error {
		calls++
		if calls < 2 {
			return errors.New("not yet")
		}
		return nil
	}, Options{Attempts: 3, Delay: time.Microsecond})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestRetryBackoff_AllFail(t *testing.T) {
	sentinel := errors.New("always")
	err := RetryBackoff(func() error { return sentinel }, Options{Attempts: 2, Delay: time.Microsecond})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
}

func TestJitter_InRange(t *testing.T) {
	d := 100 * time.Millisecond
	for i := 0; i < 100; i++ {
		j := Jitter(d)
		lo := d / 2   // d - 50% = 50ms
		hi := d + d/2 // d + 50% = 150ms (exclusive upper from rand)
		if j < lo || j > hi {
			t.Errorf("Jitter(%v) = %v, expected in [%v, %v]", d, j, lo, hi)
		}
	}
}
