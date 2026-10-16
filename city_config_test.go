package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadWatchlistMigratesLegacyConfigAndPrefersUserConfig(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config", "cities.json")
	legacyPath := filepath.Join(directory, "cities.json")
	defaults := []city{{Name: "Default", Timezone: "UTC"}}
	legacy := []city{{Name: "Legacy", Timezone: "Europe/London"}}
	contents, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadWatchlist(configPath, legacyPath, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Name != "Legacy" {
		t.Fatalf("loaded legacy cities = %#v", loaded)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("legacy config was not migrated: %v", err)
	}

	userConfig := []city{{Name: "User", Timezone: "Asia/Tokyo"}}
	if err := saveCities(configPath, userConfig); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadWatchlist(configPath, legacyPath, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Name != "User" {
		t.Fatalf("user config did not take precedence: %#v", loaded)
	}
}

func TestLoadWatchlistUsesDefaultsAndRejectsMalformedConfig(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config", "cities.json")
	legacyPath := filepath.Join(directory, "missing.json")
	defaults := []city{{Name: "Default", Timezone: "UTC"}}

	loaded, err := loadWatchlist(configPath, legacyPath, defaults)
	if err != nil || len(loaded) != 1 || loaded[0].Name != "Default" {
		t.Fatalf("default fallback: cities=%#v err=%v", loaded, err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadWatchlist(configPath, legacyPath, defaults); err == nil {
		t.Fatal("malformed user config was silently replaced")
	}
}

func TestSaveCitiesRoundTripsAndReportsFilesystemErrors(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config", "cities.json")
	want := []city{{GeoNameID: 123, Name: "London", Country: "United Kingdom", Timezone: "Europe/London"}}
	if err := saveCities(configPath, want); err != nil {
		t.Fatal(err)
	}
	got, err := readCities(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("round-trip cities = %#v, want %#v", got, want)
	}

	blockingPath := filepath.Join(directory, "not-a-directory")
	if err := os.WriteFile(blockingPath, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveCities(filepath.Join(blockingPath, "cities.json"), want); err == nil {
		t.Fatal("save unexpectedly succeeded through a regular file")
	}
}

func TestWatchlistRoundTripsSortModeAndLoadsLegacyArrays(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config", "cities.json")
	want := watchlistConfig{
		Cities:           []city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}},
		Sort:             sortOffsetDescending,
		ShowUTC:          true,
		DaylightTimezone: "America/Los_Angeles",
		DaylightCityName: "Los Angeles",
		DaylightMode:     daylightModeRolling,
		Columns: []columnPreference{
			{ID: columnDaylight, Visible: true},
			{ID: columnTime, Visible: false},
			{ID: columnDate, Visible: true},
			{ID: columnDay, Visible: true},
			{ID: columnUTCOffset, Visible: false},
			{ID: columnTimezone, Visible: true},
		},
		TimezoneNameStyle: timezoneNameIANA,
		DateFormat:        defaultDateLayout,
		TimeFormat:        defaultTimeLayout,
	}
	if err := saveWatchlist(configPath, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadWatchlistConfig(configPath, filepath.Join(directory, "legacy.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sort != want.Sort || got.ShowUTC != want.ShowUTC || got.DaylightTimezone != want.DaylightTimezone || got.DaylightCityName != want.DaylightCityName || got.DaylightMode != want.DaylightMode || got.TimezoneNameStyle != want.TimezoneNameStyle || got.DateFormat != want.DateFormat || got.TimeFormat != want.TimeFormat || len(got.Cities) != 1 || got.Cities[0] != want.Cities[0] || !reflect.DeepEqual(got.Columns, want.Columns) {
		t.Fatalf("watchlist=%#v, want %#v", got, want)
	}

	legacyPath := filepath.Join(directory, "legacy.json")
	legacyContents, err := json.Marshal(want.Cities)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, legacyContents, 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := loadWatchlistConfig(filepath.Join(directory, "new", "cities.json"), legacyPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Sort != sortNameAscending || legacy.ShowUTC || legacy.DaylightTimezone != "UTC" || legacy.DaylightCityName != "UTC" || legacy.DaylightMode != daylightModeCalendar || legacy.TimezoneNameStyle != timezoneNameIANA || legacy.DateFormat != defaultDateLayout || legacy.TimeFormat != defaultTimeLayout || len(legacy.Cities) != 1 || !reflect.DeepEqual(legacy.Columns, defaultColumnPreferences()) {
		t.Fatalf("legacy config=%#v, want name ascending default", legacy)
	}
	if err := os.WriteFile(configPath, []byte(`{"cities":[],"sort":"name_asc"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	omitted, err := readWatchlist(configPath)
	if err != nil || omitted.ShowUTC || omitted.DaylightTimezone != "UTC" || omitted.DaylightCityName != "UTC" || omitted.DaylightMode != daylightModeCalendar || omitted.TimezoneNameStyle != timezoneNameIANA || omitted.DateFormat != defaultDateLayout || omitted.TimeFormat != defaultTimeLayout || !reflect.DeepEqual(omitted.Columns, defaultColumnPreferences()) {
		t.Fatalf("watchlist omitted preferences=%#v err=%v, want current defaults", omitted, err)
	}
	if err := os.WriteFile(configPath, []byte(`{"cities":[],"daylight_timezone":"Invalid/Zone","daylight_city_name":"Broken"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid, err := readWatchlist(configPath)
	if err != nil || invalid.DaylightTimezone != "UTC" || invalid.DaylightCityName != "UTC" {
		t.Fatalf("invalid daylight reference=%q (%q), err=%v; want UTC defaults", invalid.DaylightTimezone, invalid.DaylightCityName, err)
	}
	if err := os.WriteFile(configPath, []byte(`{"cities":[],"daylight_mode":"unknown"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	invalidMode, err := readWatchlist(configPath)
	if err != nil || invalidMode.DaylightMode != daylightModeCalendar {
		t.Fatalf("invalid daylight mode=%q, err=%v; want calendar mode", invalidMode.DaylightMode, err)
	}
}
