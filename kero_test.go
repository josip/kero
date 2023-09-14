package kero

import (
	"testing"
)

func TestNewWithNoOptions(t *testing.T) {
	if _, err := New(); err == nil {
		t.Error("should fail with no options")
	}
}

func TestNewWithDBPath(t *testing.T) {
	if _, err := New(WithDB(t.TempDir())); err != nil {
		t.Error("should succeed with only DB path")
	}
}

func TestNewWithInvalidDashboardPath(t *testing.T) {
	if _, err := New(WithDB(t.TempDir()), WithDashboardPath("/")); err == nil {
		t.Error("should fail with invalid dashboard path")
	}
}

func TestWithMultipleOptions(t *testing.T) {
	k, err := New(
		WithDB(t.TempDir()),
		WithBotsIgnored(true),
		WithDashboardPath("/kero"),
	)
	if err != nil {
		t.Error("should succeed with DB path provided")
	}
	if k.IgnoreBots != true {
		t.Error("IgnoreBots option not applied")
	}
	if k.DashboardPath != "/kero" {
		t.Error("DashboardPath option not applied")
	}
}

func TestNewWithInvalidGeoIP(t *testing.T) {
	tempDir := t.TempDir()
	_, err := New(
		WithDB(tempDir),
		WithGeoIPDB(tempDir+"/not-there.mmdb"),
	)
	if err == nil {
		t.Fatal("invalid GeoIP DB should not be accepted")
	}
}
