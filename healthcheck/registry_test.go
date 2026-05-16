package healthcheck_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paper-board/sdk/healthcheck"
)

func TestRegistry_EmptyReturns200(t *testing.T) {
	r := healthcheck.New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Status string                        `json:"status"`
		Checks map[string]healthcheck.Result `json:"checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ready" {
		t.Fatalf("expected 'ready', got %q", body.Status)
	}
}

type failingChecker struct{}

func (failingChecker) Name() string                    { return "always-fail" }
func (failingChecker) Check(ctx context.Context) error { return errors.New("nope") }

func TestRegistry_FailingCheckerReturns503(t *testing.T) {
	r := healthcheck.New(failingChecker{})
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 503 {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}

	var body struct {
		Status string                        `json:"status"`
		Checks map[string]healthcheck.Result `json:"checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "not_ready" {
		t.Fatalf("expected 'not_ready', got %q", body.Status)
	}
	if body.Checks["always-fail"].Status != "fail" {
		t.Fatalf("expected 'fail' status for always-fail checker")
	}
}
