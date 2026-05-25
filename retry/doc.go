// Package retry provides exponential backoff with jitter for paper-board
// service gRPC call sites. It is the inner tier of the two-layer retry
// strategy (D9.4): sdk/retry handles transient blips at the call site;
// the outbox publish retry (sdk/outbox) is the outer tier.
//
// # Usage
//
//	ctx = retry.WithTarget(ctx, "vaults.GetCredential")
//	err := retry.Do(ctx, retry.DefaultPolicy, func(ctx context.Context) error {
//	    resp, err = vaultsClient.GetCredential(ctx, req)
//	    return err
//	})
//
// # Retry decision
//
// Per attempt:
//   - IsRetryable(err) → schedule retry (subject to MaxAttempts)
//   - IsPermanent(err) → return immediately, no retry
//   - ClassificationUnknown → treated as Permanent (fail-loud, no silent retries)
//
// # Jitter
//
// Mandatory ±20% jitter (D9.4) prevents thundering herd when many services
// retry against the same downstream.
package retry
