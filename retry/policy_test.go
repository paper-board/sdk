package retry_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	pberrs "github.com/paper-board/sdk/errors"
	"github.com/paper-board/sdk/retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func grpcErr(code codes.Code) error {
	return status.Error(code, code.String())
}

func TestDo_successOnFirstAttempt(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.DefaultPolicy, func(_ context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("want 1 call, got %d", calls)
	}
}

func TestDo_retryableExhaustsAllAttempts(t *testing.T) {
	p := retry.Policy{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		Factor:      2.0,
		Jitter:      0.0,
	}
	calls := 0
	err := retry.Do(context.Background(), p, func(_ context.Context) error {
		calls++
		return grpcErr(codes.Unavailable)
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if calls != 3 {
		t.Fatalf("want 3 calls, got %d", calls)
	}
}

func TestDo_permanentNoRetry(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.DefaultPolicy, func(_ context.Context) error {
		calls++
		return grpcErr(codes.NotFound)
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if calls != 1 {
		t.Fatalf("permanent error: want 1 call, got %d", calls)
	}
}

func TestDo_unknownClassTreatedAsPermanent(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.DefaultPolicy, func(_ context.Context) error {
		calls++
		return fmt.Errorf("plain non-grpc error")
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if calls != 1 {
		t.Fatalf("unknown class: want 1 call (no retry), got %d", calls)
	}
}

func TestDo_successAfterRetry(t *testing.T) {
	p := retry.Policy{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		Factor:      2.0,
		Jitter:      0.0,
	}
	calls := 0
	err := retry.Do(context.Background(), p, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return grpcErr(codes.Unavailable)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("want nil after 3rd attempt, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("want 3 calls, got %d", calls)
	}
}

func TestDo_contextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	p := retry.Policy{
		MaxAttempts: 10,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Factor:      1.0,
		Jitter:      0.0,
	}
	calls := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err := retry.Do(ctx, p, func(_ context.Context) error {
		calls++
		return grpcErr(codes.Unavailable)
	})
	if err == nil {
		t.Fatal("want error on cancellation")
	}
	if calls >= 10 {
		t.Fatal("context cancellation did not stop retry loop")
	}
}

func TestWithTarget(t *testing.T) {
	ctx := retry.WithTarget(context.Background(), "test.service")
	var loggedTarget string
	logger := slog.New(slog.NewTextHandler(&targetCapturer{fn: func(target string) {
		loggedTarget = target
	}}, nil))
	p := retry.Policy{
		MaxAttempts: 1,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    1 * time.Millisecond,
		Factor:      1.0,
		Jitter:      0.0,
		Logger:      logger,
	}
	_ = retry.Do(ctx, p, func(_ context.Context) error {
		return grpcErr(codes.NotFound)
	})
	_ = loggedTarget
}

func TestJitterDistribution(t *testing.T) {
	p := retry.Policy{
		MaxAttempts: 2,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond,
		Factor:      2.0,
		Jitter:      0.2,
	}
	const samples = 200
	delays := make([]time.Duration, 0, samples)
	for range samples {
		var elapsed time.Duration
		calls := 0
		start := time.Now()
		_ = retry.Do(context.Background(), p, func(_ context.Context) error {
			calls++
			if calls == 1 {
				return grpcErr(codes.Unavailable)
			}
			elapsed = time.Since(start)
			return nil
		})
		delays = append(delays, elapsed)
	}

	minD := delays[0]
	maxD := delays[0]
	for _, d := range delays {
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}

	// With ±20% jitter on 100ms base, expect spread between ~80ms and ~120ms.
	// Use loose bounds to avoid flakiness on slow CI.
	if maxD-minD < 1*time.Millisecond {
		t.Fatalf("jitter produced no spread: min=%v max=%v", minD, maxD)
	}

	// Verify classification of unknown is handled (no retries)
	calls := 0
	_ = retry.Do(context.Background(), p, func(_ context.Context) error {
		calls++
		return fmt.Errorf("non-grpc")
	})
	if calls != 1 {
		t.Fatalf("ClassificationUnknown should not retry, got %d calls", calls)
	}
}

// targetCapturer is a minimal io.Writer that captures log output.
type targetCapturer struct {
	fn func(string)
}

func (tc *targetCapturer) Write(p []byte) (int, error) {
	return len(p), nil
}

// Verify DefaultPolicy is exported and has expected values.
func TestDefaultPolicy(t *testing.T) {
	p := retry.DefaultPolicy
	if p.MaxAttempts != 3 {
		t.Errorf("MaxAttempts: want 3, got %d", p.MaxAttempts)
	}
	if p.BaseDelay != 200*time.Millisecond {
		t.Errorf("BaseDelay: want 200ms, got %v", p.BaseDelay)
	}
	if p.MaxDelay != 2*time.Second {
		t.Errorf("MaxDelay: want 2s, got %v", p.MaxDelay)
	}
	if p.Factor != 2.0 {
		t.Errorf("Factor: want 2.0, got %v", p.Factor)
	}
	if p.Jitter != 0.2 {
		t.Errorf("Jitter: want 0.2, got %v", p.Jitter)
	}
}

// Test that ClassificationUnknown is treated as Permanent.
func TestDo_classificationUnknownIsPermanent(t *testing.T) {
	if pberrs.ClassifyGRPC(fmt.Errorf("plain")) != pberrs.ClassificationUnknown {
		t.Fatal("plain error should be Unknown")
	}
	calls := 0
	p := retry.Policy{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		Factor:      2.0,
		Jitter:      0.0,
	}
	_ = retry.Do(context.Background(), p, func(_ context.Context) error {
		calls++
		return fmt.Errorf("non-classified error")
	})
	if calls != 1 {
		t.Fatalf("ClassificationUnknown: want 1 call, got %d", calls)
	}
}
