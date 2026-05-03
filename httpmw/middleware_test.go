package httpmw_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/paper-board/sdk/httpmw"
	pblog "github.com/paper-board/sdk/log"
)

func captureLogger() (*bytes.Buffer, func()) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &buf, func() { slog.SetDefault(prev) }
}

func TestRequestIDGenerated(t *testing.T) {
	var got string
	h := httpmw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = pblog.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(rec, req)

	if got == "" {
		t.Fatal("request_id not stored in ctx")
	}
	if rec.Header().Get(httpmw.HeaderRequestID) != got {
		t.Fatalf("response header mismatch: %q vs ctx %q", rec.Header().Get(httpmw.HeaderRequestID), got)
	}
}

func TestRequestIDPassThrough(t *testing.T) {
	var got string
	h := httpmw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = pblog.RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(httpmw.HeaderRequestID, "client-supplied-123")
	h.ServeHTTP(rec, req)
	if got != "client-supplied-123" {
		t.Fatalf("got %q", got)
	}
	if rec.Header().Get(httpmw.HeaderRequestID) != "client-supplied-123" {
		t.Fatalf("header echoed: %q", rec.Header().Get(httpmw.HeaderRequestID))
	}
}

func TestTrustHeadersWires(t *testing.T) {
	var orgID, userID string
	var roles []string
	h := httpmw.TrustHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID = pblog.OrgFromContext(r.Context())
		userID = pblog.UserFromContext(r.Context())
		roles = pblog.RolesFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(httpmw.HeaderOrgID, "org-1")
	req.Header.Set(httpmw.HeaderUserID, "user-2")
	req.Header.Set(httpmw.HeaderRoles, "admin, billing , reader")
	h.ServeHTTP(rec, req)

	if orgID != "org-1" || userID != "user-2" {
		t.Fatalf("ctx: org=%q user=%q", orgID, userID)
	}
	if len(roles) != 3 || roles[0] != "admin" || roles[2] != "reader" {
		t.Fatalf("roles: %v", roles)
	}
}

func TestTrustHeadersEmptyNoOp(t *testing.T) {
	var orgID string
	h := httpmw.TrustHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID = pblog.OrgFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(rec, req)
	if orgID != "" {
		t.Fatalf("expected empty org, got %q", orgID)
	}
}

func TestBodyLimitRejectsOversized(t *testing.T) {
	limit := int64(8)
	called := false
	h := httpmw.BodyLimit(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, err := io.ReadAll(r.Body)
		if err == nil {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		}
	}))
	body := strings.NewReader("0123456789")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", body)
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler not called")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestBodyLimitAllowsUnderSize(t *testing.T) {
	h := httpmw.BodyLimit(64)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader("hi"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestRecover(t *testing.T) {
	buf, restore := captureLogger()
	defer restore()

	mw := httpmw.Recover("agents")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
	logs := buf.String()
	if !strings.Contains(logs, "panic recovered") || !strings.Contains(logs, "error.kind") {
		t.Fatalf("missing log keys: %s", logs)
	}
}

func TestLogger(t *testing.T) {
	buf, restore := captureLogger()
	defer restore()

	h := httpmw.Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hi"))
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/foo", nil)
	h.ServeHTTP(rec, req)
	logs := buf.String()
	for _, key := range []string{"http_request", "method", "route", "status", "duration_ms", "bytes_in", "bytes_out"} {
		if !strings.Contains(logs, key) {
			t.Fatalf("missing key %q in log: %s", key, logs)
		}
	}
	if !strings.Contains(logs, `"status":418`) {
		t.Fatalf("status not recorded: %s", logs)
	}
}

func TestAuthStub(t *testing.T) {
	org := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	var got string
	mw := httpmw.AuthStub(org)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = pblog.OrgFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(rec, req)
	if got != org.String() {
		t.Fatalf("org: %q want %q", got, org.String())
	}
}

func TestOtelHTTP(t *testing.T) {
	mw := httpmw.OtelHTTP("agents")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
}

var _ = context.TODO
