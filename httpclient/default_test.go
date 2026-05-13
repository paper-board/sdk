package httpclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/paper-board/sdk/httpclient"
)

func TestDefaultGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := httpclient.Default(2 * time.Second)
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 204 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestDefaultFreshPerCall(t *testing.T) {
	a := httpclient.Default(time.Second)
	b := httpclient.Default(time.Second)
	if a == b {
		t.Fatal("Default should return a fresh client per call")
	}
	if a.Timeout != time.Second {
		t.Fatalf("timeout: %v", a.Timeout)
	}
}
