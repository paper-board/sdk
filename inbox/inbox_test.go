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

func TestNewInboxReturnsNonNil(t *testing.T) {
	i := New(nil, "test_schema")
	if i == nil {
		t.Fatal("expected non-nil Inbox")
	}
}
