// Package inbox provides consumer-side event deduplication for paper-board
// async event consumers. Consumers track processed event_ids in a
// processed_events table to avoid re-applying the same envelope (D9.2).
//
// # Usage
//
//	pool, _ := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
//
//	// Run migration once at startup:
//	helper := inbox.NewMigrationHelper("onboarding")
//	if err := helper(ctx, pool); err != nil { ... }
//
//	ib := inbox.New(pool, "onboarding")
//
//	// In the event handler — claim and apply atomically:
//	tx, _ := pool.Begin(ctx)
//	claimed, err := ib.TryMark(ctx, tx, event.EventID, event.EventType, event.OrgID)
//	if err != nil { tx.Rollback(ctx); return err }
//	if !claimed { tx.Rollback(ctx); return nil } // duplicate — skip
//	// ... apply business state inside the same tx ...
//	tx.Commit(ctx)
//
// # Retention
//
// processed_events rows are retained for 180 days (D9.2). Cleanup is handled
// by a per-service k8s CronJob declared in each consumer's Helm chart;
// the sdk does not manage cleanup.
package inbox
