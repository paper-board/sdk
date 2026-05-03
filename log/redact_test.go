package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	pblog "github.com/paper-board/sdk/log"
)

func newCapture() (*bytes.Buffer, *slog.Logger) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	red := pblog.NewRedactingHandler(inner, pblog.RedactOpts{})
	return &buf, slog.New(red)
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, string(b))
	}
	return m
}

func TestRedactDenyKey(t *testing.T) {
	buf, logger := newCapture()
	logger.Info("login", "username", "alice", "password", "hunter2", "api_key", "sk-abc123def456ghi789jkl")
	m := decode(t, buf.Bytes())
	if m["password"] != pblog.REDACT {
		t.Fatalf("password not redacted: %v", m["password"])
	}
	if m["api_key"] != pblog.REDACT {
		t.Fatalf("api_key not redacted: %v", m["api_key"])
	}
	if m["username"] != "alice" {
		t.Fatalf("username should pass: %v", m["username"])
	}
}

func TestRedactBearerToken(t *testing.T) {
	buf, logger := newCapture()
	logger.Info("auth", "header", "Bearer abc.def.ghi-jkl_mno")
	m := decode(t, buf.Bytes())
	got, _ := m["header"].(string)
	if !strings.Contains(got, pblog.REDACT) || strings.Contains(got, "abc.def") {
		t.Fatalf("bearer not redacted: %q", got)
	}
}

func TestRedactJWT(t *testing.T) {
	buf, logger := newCapture()
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.signaturepart"
	logger.Info("got jwt: " + jwt)
	m := decode(t, buf.Bytes())
	msg, _ := m["msg"].(string)
	if strings.Contains(msg, "eyJ") {
		t.Fatalf("jwt not redacted from message: %q", msg)
	}
}

func TestRedactEmail(t *testing.T) {
	buf, logger := newCapture()
	logger.Info("hi", "from", "alice@example.com")
	m := decode(t, buf.Bytes())
	if m["from"] == "alice@example.com" {
		t.Fatalf("email not redacted: %v", m["from"])
	}
}

func TestRedactWithAttrsPreRedacts(t *testing.T) {
	buf, logger := newCapture()
	scoped := logger.With("authorization", "Bearer xyz")
	scoped.Info("hit")
	m := decode(t, buf.Bytes())
	if m["authorization"] != pblog.REDACT {
		t.Fatalf("WithAttrs not redacted: %v", m["authorization"])
	}
}

func TestRedactPassthroughNonString(t *testing.T) {
	buf, logger := newCapture()
	logger.Info("counts", "n", 42, "ok", true)
	m := decode(t, buf.Bytes())
	if m["n"].(float64) != 42 || m["ok"] != true {
		t.Fatalf("non-string mangled: %v", m)
	}
}

func TestRedactCustomDeny(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	red := pblog.NewRedactingHandler(inner, pblog.RedactOpts{DenyKeys: []string{"ssn"}})
	logger := slog.New(red)
	logger.Info("x", "ssn", "111-22-3333")
	m := decode(t, buf.Bytes())
	if m["ssn"] != pblog.REDACT {
		t.Fatalf("custom deny not honored: %v", m["ssn"])
	}
}

func TestRedactEnabled(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	red := pblog.NewRedactingHandler(inner, pblog.RedactOpts{})
	if red.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Enabled(Info) should be false when inner is Warn-only")
	}
	if !red.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("Enabled(Error) should be true")
	}
}
