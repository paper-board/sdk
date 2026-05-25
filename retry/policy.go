package retry

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/paper-board/sdk/errors"
)

// Policy controls retry behaviour.
type Policy struct {
	// MaxAttempts is the total number of attempts (first + retries).
	// Default: 3 (D9.4 inner tier).
	MaxAttempts int
	// BaseDelay is the delay before the second attempt.
	// Default: 200ms.
	BaseDelay time.Duration
	// MaxDelay caps the computed delay.
	// Default: 2s.
	MaxDelay time.Duration
	// Factor is the exponential growth multiplier.
	// Default: 2.0.
	Factor float64
	// Jitter is the ±fraction applied to each delay.
	// Default: 0.2 (±20%, mandatory per D9.4).
	Jitter float64
	// Logger emits one INFO line per attempt with retry context.
	// Uses slog.Default() if nil.
	Logger *slog.Logger
}

// DefaultPolicy is the D9.4 inner-tier configuration.
var DefaultPolicy = Policy{
	MaxAttempts: 3,
	BaseDelay:   200 * time.Millisecond,
	MaxDelay:    2 * time.Second,
	Factor:      2.0,
	Jitter:      0.2,
}

type ctxKey struct{}

// WithTarget attaches a target label (e.g. "vaults.GetCredential") to ctx so
// Do can include it in retry log lines.
func WithTarget(ctx context.Context, target string) context.Context {
	return context.WithValue(ctx, ctxKey{}, target)
}

func targetFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// Do runs fn under the policy and returns the last error if all attempts fail.
// Cancellation via ctx.Done() aborts the loop and returns ctx.Err().
func Do(ctx context.Context, p Policy, fn func(context.Context) error) error {
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	target := targetFromCtx(ctx)
	maxAttempts := p.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultPolicy.MaxAttempts
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		class := errors.ClassifyGRPC(lastErr)

		if class == errors.ClassificationPermanent || class == errors.ClassificationUnknown {
			logAttempt(logger, attempt, maxAttempts, class, lastErr, 0, target, true)
			return lastErr
		}

		if attempt == maxAttempts {
			logAttempt(logger, attempt, maxAttempts, class, lastErr, 0, target, true)
			return lastErr
		}

		delay := computeDelay(p, attempt)
		logAttempt(logger, attempt, maxAttempts, class, lastErr, delay, target, false)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

func computeDelay(p Policy, attempt int) time.Duration {
	base := p.BaseDelay
	factor := p.Factor
	if factor <= 0 {
		factor = DefaultPolicy.Factor
	}
	maxDelay := p.MaxDelay
	if maxDelay <= 0 {
		maxDelay = DefaultPolicy.MaxDelay
	}
	jitter := p.Jitter

	delay := float64(base)
	for i := 1; i < attempt; i++ {
		delay *= factor
	}
	if time.Duration(delay) > maxDelay {
		delay = float64(maxDelay)
	}

	// ±jitter: multiply by (1 + rand[-1,1] * jitter)
	if jitter > 0 {
		noise := (rand.Float64()*2 - 1) * jitter
		delay = delay * (1 + noise)
		if delay < 0 {
			delay = 0
		}
	}

	return time.Duration(delay)
}

func logAttempt(logger *slog.Logger, attempt, maxAttempts int, class errors.Classification, err error, delay time.Duration, target string, final bool) {
	classStr := classificationString(class)
	args := []any{
		"attempt", attempt,
		"error_class", classStr,
		"delay_ms", delay.Milliseconds(),
		"final", final,
	}
	if target != "" {
		args = append(args, "target", target)
	}
	logger.Info("retry attempt", args...)
}

func classificationString(c errors.Classification) string {
	switch c {
	case errors.ClassificationRetryable:
		return "retryable"
	case errors.ClassificationPermanent:
		return "permanent"
	default:
		return "unknown"
	}
}
