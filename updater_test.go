package main

import (
	"testing"
)

func TestVersionComparisonHandlesReleaseTags(t *testing.T) {
	if compareVersions("v1.2.0", "v1.2.1") != -1 {
		t.Fatal("older release unexpectedly reported as newer")
	}
	if compareVersions("v1.2.1", "v1.2.0") != 1 {
		t.Fatal("newer release unexpectedly reported as older")
	}
	if compareVersions("v1.2.1", "v1.2.1") != 0 {
		t.Fatal("matching release versions should compare equal")
	}
	if compareVersions("dev", "v1.2.1") != 0 {
		t.Fatal("local dev builds should not be treated as newer than released versions")
	}
}

func TestCurrentVersionUsesInjectedBuildVersion(t *testing.T) {
	originalVersion := appVersion
	t.Cleanup(func() { appVersion = originalVersion })
	appVersion = "v1.2.3"

	if got := currentVersion(); got != "v1.2.3" {
		t.Fatalf("currentVersion() = %q, want injected version", got)
	}
}

func TestCurrentVersionFallsBackToDevelopmentVersion(t *testing.T) {
	originalVersion := appVersion
	t.Cleanup(func() { appVersion = originalVersion })
	appVersion = "dev"

	if got := currentVersion(); got != "dev" {
		t.Fatalf("currentVersion() = %q, want dev", got)
	}
}
