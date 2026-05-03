// Package config provides an env-only configuration loader for paper-board services.
//
// MustBind reads environment variables onto a service-defined struct via
// reflection over `env`, `default`, and `validate` struct tags. After binding,
// validator/v10 runs Struct() on the destination. Required fields with no env
// or default cause MustBind to log to stderr and exit non-zero.
//
// Flags and config files are intentionally unsupported. Service binaries MAY
// accept positional/flag args via cobra for one-shot operations (e.g. migrator
// subcommands), but those flags MUST NOT override Config fields.
//
// Example:
//
//	type Config struct {
//	    DatabaseURL string        `env:"DATABASE_URL" validate:"required,url"`
//	    ServerPort  int           `env:"SERVER_PORT" default:"8080" validate:"gte=1,lte=65535"`
//	    LogLevel    string        `env:"LOG_LEVEL" default:"info" validate:"oneof=debug info warn error"`
//	    Timeout     time.Duration `env:"TIMEOUT" default:"30s"`
//	    Tags        []string      `env:"TAGS" default:"a,b,c"`
//	    Debug       bool          `env:"DEBUG" default:"false"`
//	}
//
//	cfg := config.MustBind[Config]()
package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// MustBind populates a fresh T from environment variables, applies struct-tag
// defaults for unset env vars, then runs validator/v10. On any error it logs to
// stderr and calls os.Exit(1). T MUST be a struct type.
func MustBind[T any]() *T {
	var dst T
	v := reflect.ValueOf(&dst).Elem()
	if v.Kind() != reflect.Struct {
		exit("config.MustBind: T must be a struct type")
	}
	if err := bindStruct(v); err != nil {
		exit(fmt.Sprintf("config.MustBind: %v", err))
	}
	if err := validator.New().Struct(&dst); err != nil {
		exit(fmt.Sprintf("config.MustBind: validation failed: %v", err))
	}
	return &dst
}

func bindStruct(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		env := f.Tag.Get("env")
		if env == "" {
			continue
		}
		raw, ok := os.LookupEnv(env)
		if !ok || raw == "" {
			raw = f.Tag.Get("default")
		}
		if raw == "" {
			continue
		}
		if err := setField(v.Field(i), raw); err != nil {
			return fmt.Errorf("field %s (env %s): %w", f.Name, env, err)
		}
	}
	return nil
}

func setField(fv reflect.Value, raw string) error {
	if fv.Type() == reflect.TypeOf(time.Duration(0)) {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("parse duration: %w", err)
		}
		fv.SetInt(int64(d))
		return nil
	}
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("parse bool: %w", err)
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("parse int: %w", err)
		}
		fv.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("parse uint: %w", err)
		}
		fv.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("parse float: %w", err)
		}
		fv.SetFloat(f)
	case reflect.Slice:
		if fv.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported slice element kind %s (only []string)", fv.Type().Elem().Kind())
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		fv.Set(reflect.ValueOf(out))
	default:
		return fmt.Errorf("unsupported kind %s", fv.Kind())
	}
	return nil
}

var exit = func(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
