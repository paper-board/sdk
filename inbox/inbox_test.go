package inbox

import (
	"testing"
)

func TestNewMigrationHelperReturnsFunc(t *testing.T) {
	helper := newMigrationHelper("test_schema")
	if helper == nil {
		t.Fatal("expected non-nil migration helper")
	}
}

func TestNewInboxPanicsOnNilPool(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil pool")
		}
	}()
	New(nil, "test_schema")
}

func TestNewInboxPanicsOnInvalidSchema(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on invalid schema")
		}
	}()
	New(nil, "invalid-schema!")
}

func TestValidateSchemaIdentValid(t *testing.T) {
	for _, s := range []string{"identity", "test_svc", "a", "_foo", "foo123"} {
		if err := validateSchemaIdent(s); err != nil {
			t.Errorf("schema %q should be valid: %v", s, err)
		}
	}
}

func TestValidateSchemaIdentInvalid(t *testing.T) {
	for _, s := range []string{"", "My-Schema", "123foo", "foo bar", "foo.bar"} {
		if err := validateSchemaIdent(s); err == nil {
			t.Errorf("schema %q should be invalid", s)
		}
	}
}
