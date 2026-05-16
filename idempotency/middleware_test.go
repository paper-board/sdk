package idempotency_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/paper-board/sdk/idempotency"
)

func TestMiddleware_RetryReplaysResponse(t *testing.T) {
	store := idempotency.NewMemoryStore()
	orgID := uuid.New()

	executions := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executions++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"new"}`))
	})

	mw := idempotency.Require(store)(handler)

	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/v1/resource", bytes.NewBufferString(`{"name":"foo"}`))
		r.Header.Set("Idempotency-Key", "test-key-1")
		r = r.WithContext(idempotency.WithOrgID(context.Background(), orgID))
		return r
	}

	w1 := httptest.NewRecorder()
	mw.ServeHTTP(w1, req())
	if w1.Code != 201 {
		t.Fatalf("first call: expected 201, got %d", w1.Code)
	}
	if executions != 1 {
		t.Fatalf("expected 1 execution, got %d", executions)
	}

	w2 := httptest.NewRecorder()
	mw.ServeHTTP(w2, req())
	if w2.Code != 201 {
		t.Fatalf("retry: expected 201 (replay), got %d", w2.Code)
	}
	if executions != 1 {
		t.Fatalf("expected handler NOT re-executed, got %d total executions", executions)
	}
	if w2.Header().Get("Idempotent-Replay") != "true" {
		t.Fatal("expected Idempotent-Replay header on replay")
	}
}

func TestMiddleware_BodyMismatchReturns422(t *testing.T) {
	store := idempotency.NewMemoryStore()
	orgID := uuid.New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	})
	mw := idempotency.Require(store)(handler)

	r1 := httptest.NewRequest(http.MethodPost, "/v1/resource", bytes.NewBufferString(`{"name":"a"}`))
	r1.Header.Set("Idempotency-Key", "same-key")
	r1 = r1.WithContext(idempotency.WithOrgID(context.Background(), orgID))
	mw.ServeHTTP(httptest.NewRecorder(), r1)

	r2 := httptest.NewRequest(http.MethodPost, "/v1/resource", bytes.NewBufferString(`{"name":"b"}`))
	r2.Header.Set("Idempotency-Key", "same-key")
	r2 = r2.WithContext(idempotency.WithOrgID(context.Background(), orgID))
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r2)

	if w.Code != 422 {
		t.Fatalf("expected 422 conflict, got %d", w.Code)
	}
}

func TestMiddleware_ExcludedRouteBypasses(t *testing.T) {
	store := idempotency.NewMemoryStore()
	orgID := uuid.New()
	executions := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executions++
		w.WriteHeader(200)
	})
	mw := idempotency.Require(store, idempotency.WithExclude("POST /v1/auth/login"))(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewBufferString(`{}`))
	req.Header.Set("Idempotency-Key", "ignored")
	req = req.WithContext(idempotency.WithOrgID(context.Background(), orgID))

	mw.ServeHTTP(httptest.NewRecorder(), req)
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if executions != 2 {
		t.Fatalf("excluded route should not replay; expected 2 executions, got %d", executions)
	}
}
