// Package outbox implements the transactional outbox pattern for paper-board
// services. Service code writes events into the outbox table within the same
// Postgres transaction that mutates business state; an in-process drain
// goroutine delivers unpublished rows to Redis Streams asynchronously.
//
// # Usage
//
//	pool, _ := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
//	pub, err := outbox.NewPublisher(pool, outbox.Config{
//	    Schema:        "identity",
//	    SourceService: "identity",
//	    Stream:        "paperboard.identity",
//	    RedisAddr:     "redis-master.paper-board:6379",
//	})
//	if err != nil { ... }
//
//	// In application startup:
//	go pub.Start(ctx)
//
//	// Inside a business transaction:
//	tx, _ := pool.Begin(ctx)
//	// ... mutate business tables ...
//	err = pub.Publish(ctx, tx, outbox.Event{
//	    EventType:     "identity.user.created",
//	    Stream:        "paperboard.identity",
//	    Payload:       payloadJSON,
//	    TraceID:       traceID,
//	    SchemaVersion: 1,
//	    OccurredAt:    time.Now().UTC(),
//	})
//	tx.Commit(ctx)
//
// The drain goroutine picks up pending rows every DrainInterval (default 200ms)
// and publishes them to Redis Streams. On persistent Redis failure (MaxAttempts
// reached) the row is promoted to dead-letter status.
//
// # Migration helper
//
// Before constructing a Publisher, run the migration helper once per service
// schema (idempotent, safe to run multiple times):
//
//	helper := outbox.NewMigrationHelper("identity")
//	if err := helper(ctx, pool); err != nil { ... }
package outbox
