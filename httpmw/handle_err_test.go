package httpmw_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"

	pberrs "github.com/paper-board/sdk/errors"
	"github.com/paper-board/sdk/httpmw"
)

func decodeEnv(t *testing.T, body []byte) httpmw.ErrorEnvelope {
	t.Helper()
	var env httpmw.ErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, string(body))
	}
	return env
}

func TestHandleErrSentinelMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
		wantStat int
	}{
		{"not_found", pberrs.Wrap(pberrs.ErrNotFound, "agent", "id", "abc"), "agents.not_found", http.StatusNotFound},
		{"conflict", pberrs.Wrap(pberrs.ErrConflict, "slug"), "agents.conflict", http.StatusConflict},
		{"unauthorized", pberrs.Wrap(pberrs.ErrUnauthorized, "x"), "agents.unauthorized", http.StatusUnauthorized},
		{"permission_denied", pberrs.Wrap(pberrs.ErrPermissionDenied, "x"), "agents.permission_denied", http.StatusForbidden},
		{"invalid_input", pberrs.Wrap(pberrs.ErrInvalidInput, "x"), "agents.invalid_input", http.StatusBadRequest},
		{"unavailable", pberrs.Wrap(pberrs.ErrUnavailable, "x"), "agents.unavailable", http.StatusServiceUnavailable},
		{"internal", pberrs.Wrap(pberrs.ErrInternal, "x"), "agents.internal", http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			httpmw.HandleErr(rec, req, "agents", c.err)
			if rec.Code != c.wantStat {
				t.Fatalf("status: got %d want %d", rec.Code, c.wantStat)
			}
			env := decodeEnv(t, rec.Body.Bytes())
			if env.Error.Code != c.wantCode {
				t.Fatalf("code: got %q want %q", env.Error.Code, c.wantCode)
			}
			if env.Error.Message == "" {
				t.Fatal("message empty")
			}
			if rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("content-type: %q", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestHandleErrNilNoOp(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	httpmw.HandleErr(rec, req, "agents", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("nil err should not write status: got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("nil err should not write body: got %q", rec.Body.String())
	}
}

func TestHandleErrValidationDetails(t *testing.T) {
	type form struct {
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"gte=18,lte=99"`
	}
	verr := validator.New().Struct(form{Email: "not-an-email", Age: 200})
	if verr == nil {
		t.Fatal("expected validation error from fixture")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)
	httpmw.HandleErr(rec, req, "agents", verr)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rec.Code)
	}
	env := decodeEnv(t, rec.Body.Bytes())
	if env.Error.Code != "agents.invalid_input" {
		t.Fatalf("code: %q", env.Error.Code)
	}
	details, ok := env.Error.Details.([]any)
	if !ok || len(details) == 0 {
		t.Fatalf("details: %v", env.Error.Details)
	}
}

func TestHandleErrLeaksOnlySentinelMessage(t *testing.T) {
	secret := "secret-internal-detail"
	err := pberrs.Wrap(pberrs.ErrInternal, secret)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	httpmw.HandleErr(rec, req, "agents", err)
	if got := rec.Body.String(); contains(got, secret) {
		t.Fatalf("response leaked wrapped detail: %s", got)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	httpmw.WriteError(rec, http.StatusTeapot, "agents.teapot", "i am a teapot")
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status: %d", rec.Code)
	}
	env := decodeEnv(t, rec.Body.Bytes())
	if env.Error.Code != "agents.teapot" || env.Error.Message != "i am a teapot" {
		t.Fatalf("envelope: %+v", env.Error)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (s[:len(sub)] == sub || contains(s[1:], sub))))
}

var _ = context.TODO
