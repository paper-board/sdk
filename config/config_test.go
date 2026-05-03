package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/paper-board/sdk/config"
)

type sample struct {
	Required    string        `env:"S_REQUIRED" validate:"required"`
	WithDefault string        `env:"S_DEF" default:"hello"`
	Port        int           `env:"S_PORT" default:"8080" validate:"gte=1,lte=65535"`
	Timeout     time.Duration `env:"S_TIMEOUT" default:"30s"`
	Debug       bool          `env:"S_DEBUG" default:"false"`
	Tags        []string      `env:"S_TAGS" default:"a,b,c"`
	unexported  string        //nolint:unused // covered by reflection skip
}

func TestMustBindHappy(t *testing.T) {
	t.Setenv("S_REQUIRED", "hi")
	t.Setenv("S_PORT", "9000")
	t.Setenv("S_TIMEOUT", "1m30s")
	t.Setenv("S_DEBUG", "true")
	t.Setenv("S_TAGS", "x, y , z")

	c := config.MustBind[sample]()

	if c.Required != "hi" {
		t.Fatalf("Required: %q", c.Required)
	}
	if c.WithDefault != "hello" {
		t.Fatalf("WithDefault: %q", c.WithDefault)
	}
	if c.Port != 9000 {
		t.Fatalf("Port: %d", c.Port)
	}
	if c.Timeout != 90*time.Second {
		t.Fatalf("Timeout: %v", c.Timeout)
	}
	if !c.Debug {
		t.Fatalf("Debug not true")
	}
	if len(c.Tags) != 3 || c.Tags[0] != "x" || c.Tags[1] != "y" || c.Tags[2] != "z" {
		t.Fatalf("Tags: %v", c.Tags)
	}
}

func TestMustBindDefaults(t *testing.T) {
	t.Setenv("S_REQUIRED", "hi")

	c := config.MustBind[sample]()

	if c.WithDefault != "hello" {
		t.Fatalf("WithDefault: %q", c.WithDefault)
	}
	if c.Port != 8080 {
		t.Fatalf("Port: %d", c.Port)
	}
	if c.Timeout != 30*time.Second {
		t.Fatalf("Timeout: %v", c.Timeout)
	}
	if c.Debug {
		t.Fatalf("Debug: true (want false)")
	}
	if len(c.Tags) != 3 {
		t.Fatalf("Tags: %v", c.Tags)
	}
}

func TestMustBindMissingRequired(t *testing.T) {
	got := captureExit(t, func() {
		_ = config.MustBind[sample]()
	})
	if !strings.Contains(got, "validation failed") {
		t.Fatalf("expected validation failure, got: %s", got)
	}
}

func TestMustBindBadDuration(t *testing.T) {
	t.Setenv("S_REQUIRED", "hi")
	t.Setenv("S_TIMEOUT", "not-a-duration")
	got := captureExit(t, func() {
		_ = config.MustBind[sample]()
	})
	if !strings.Contains(got, "parse duration") {
		t.Fatalf("expected duration parse error, got: %s", got)
	}
}

func TestMustBindRejectsNonStruct(t *testing.T) {
	got := captureExit(t, func() {
		_ = config.MustBind[int]()
	})
	if !strings.Contains(got, "must be a struct") {
		t.Fatalf("expected struct-type error, got: %s", got)
	}
}

func captureExit(t *testing.T, fn func()) (msg string) {
	t.Helper()
	prev := config.SetExitForTest(func(s string) {
		msg = s
		panic("exit-stub")
	})
	t.Cleanup(func() { config.SetExitForTest(prev) })
	defer func() { _ = recover() }()
	fn()
	return msg
}
