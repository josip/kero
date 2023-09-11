package kero

import (
	"testing"
)

func TestNewWithNoOptions(t *testing.T) {
	if _, err := New(); err == nil {
		t.Fatal("should fail with no options")
	}
}

func TestNewWithDBPath(t *testing.T) {
	if _, err := New(WithDB(t.TempDir())); err != nil {
		t.Fatal("should have succeeded with only DB path")
	}
}

func TestWithMultipleOptions(t *testing.T) {
	k, err := New(
		WithDB(t.TempDir()),
		WithBotsIgnored(true),
		WithDashboardPath("/kero"),
	)
	if err != nil {
		t.Fatal("should have succeded with DB path provided")
	}
	if k.IgnoreBots != true {
		t.Fatal("IgnoreBots option not applied")
	}
	if k.DashboardPath != "/kero" {
		t.Fatal("DashboardPath option not applied")
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
