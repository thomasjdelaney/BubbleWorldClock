package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func testModel(cities []city) model {
	return model{
		cities: cities,
		now:    time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC),
		width:  80,
		help:   help.New(),
		keys:   newKeyMap(),
	}
}

func TestCityRecordProvidesSearchableLabels(t *testing.T) {
	record := cityRecord{
		Name:        "Munich",
		ASCIIName:   "Muenchen",
		CountryCode: "DE",
		Country:     "Germany",
		Admin1Code:  "02",
		Region:      "Bavaria",
		Timezone:    "Europe/Berlin",
	}

	if record.Title() != "Munich" {
		t.Fatalf("title=%q, want city name", record.Title())
	}
	for _, searchable := range []string{"Munich", "Muenchen", "Bavaria", "Germany", "DE", "Europe/Berlin"} {
		if !strings.Contains(record.FilterValue(), searchable) {
			t.Errorf("filter value %q does not contain %q", record.FilterValue(), searchable)
		}
	}
	for _, described := range []string{"Bavaria", "Germany", "Europe/Berlin"} {
		if !strings.Contains(record.Description(), described) {
			t.Errorf("description %q does not contain %q", record.Description(), described)
		}
	}
}

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

func TestSortCitiesByNameAndOffsetPreservesSelection(t *testing.T) {
	cities := []city{
		{GeoNameID: 1, Name: "Tokyo", Timezone: "Asia/Tokyo"},
		{GeoNameID: 2, Name: "London", Timezone: "Europe/London"},
		{GeoNameID: 3, Name: "New York", Timezone: "America/New_York"},
	}
	tests := []struct {
		name string
		mode sortMode
		want []string
	}{
		{name: "name ascending", mode: sortNameAscending, want: []string{"London", "New York", "Tokyo"}},
		{name: "name descending", mode: sortNameDescending, want: []string{"Tokyo", "New York", "London"}},
		{name: "offset ascending", mode: sortOffsetAscending, want: []string{"New York", "London", "Tokyo"}},
		{name: "offset descending", mode: sortOffsetDescending, want: []string{"Tokyo", "London", "New York"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := testModel(append([]city(nil), cities...))
			m.sortMode = test.mode
			m.selected = 0
			selectedID := m.cities[m.selected].GeoNameID
			m.sortCitiesPreservingSelection()
			for index, want := range test.want {
				if m.cities[index].Name != want {
					t.Fatalf("city[%d]=%q, want %q", index, m.cities[index].Name, want)
				}
			}
			if m.cities[m.selected].GeoNameID != selectedID {
				t.Fatalf("selected city ID=%d, want %d", m.cities[m.selected].GeoNameID, selectedID)
			}
		})
	}
}

func TestUTCReferenceFollowsActiveSortOrder(t *testing.T) {
	cities := []city{
		{Name: "Tokyo", Timezone: "Asia/Tokyo"},
		{Name: "London", Timezone: "Europe/London"},
		{Name: "Zebra", Timezone: "America/New_York"},
		{Name: "Alpha", Timezone: "America/Los_Angeles"},
	}
	tests := []struct {
		name string
		mode sortMode
		want []string
	}{
		{name: "name ascending", mode: sortNameAscending, want: []string{"Alpha", "London", "Tokyo", "UTC", "Zebra"}},
		{name: "name descending", mode: sortNameDescending, want: []string{"Zebra", "UTC", "Tokyo", "London", "Alpha"}},
		{name: "offset ascending", mode: sortOffsetAscending, want: []string{"Alpha", "Zebra", "London", "UTC", "Tokyo"}},
		{name: "offset descending", mode: sortOffsetDescending, want: []string{"Tokyo", "London", "UTC", "Zebra", "Alpha"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := testModel(append([]city(nil), cities...))
			m.width = 100
			m.showUTC = true
			m.sortMode = test.mode
			m.sortCitiesPreservingSelection()

			view := m.View().Content
			lastPosition := -1
			for _, label := range test.want {
				position := strings.Index(view, label)
				if label == "UTC" {
					position = strings.LastIndex(view, label)
				}
				if position <= lastPosition {
					t.Fatalf("clock row %q appears out of order in %q", label, view)
				}
				lastPosition = position
			}
			if len(m.cities) != len(cities) {
				t.Fatalf("UTC reference changed managed city count to %d", len(m.cities))
			}
		})
	}
}

func TestSortPlacesInvalidTimezoneLast(t *testing.T) {
	m := testModel([]city{
		{Name: "Broken", Timezone: "Not/AZone"},
		{Name: "London", Timezone: "Europe/London"},
	})
	m.sortMode = sortOffsetAscending
	m.sortCitiesPreservingSelection()
	if m.cities[0].Name != "London" || m.cities[1].Name != "Broken" {
		t.Fatalf("sorted cities=%#v, want valid timezone first", m.cities)
	}
}

func TestWatchlistRoundTripsSortModeAndLoadsLegacyArrays(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config", "cities.json")
	want := watchlistConfig{
		Cities:  []city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}},
		Sort:    sortOffsetDescending,
		ShowUTC: true,
	}
	if err := saveWatchlist(configPath, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadWatchlistConfig(configPath, filepath.Join(directory, "legacy.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sort != want.Sort || got.ShowUTC != want.ShowUTC || len(got.Cities) != 1 || got.Cities[0] != want.Cities[0] {
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
	if legacy.Sort != sortNameAscending || legacy.ShowUTC || len(legacy.Cities) != 1 {
		t.Fatalf("legacy config=%#v, want name ascending default", legacy)
	}
	if err := os.WriteFile(configPath, []byte(`{"cities":[],"sort":"name_asc"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	omitted, err := readWatchlist(configPath)
	if err != nil || omitted.ShowUTC {
		t.Fatalf("watchlist without show_utc=%t err=%v, want default false", omitted.ShowUTC, err)
	}
}

func TestToggleUTCReferencePersistsAndWaitsForCurrentSave(t *testing.T) {
	m := testModel([]city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}})
	m.screen = manageScreen
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	toggle := tea.KeyPressMsg(tea.Key{Text: "u", Code: 'u'})

	updated, cmd := m.Update(toggle)
	enabled := updated.(model)
	if !enabled.showUTC || cmd == nil {
		t.Fatalf("UTC toggle: showUTC=%t cmd=%v", enabled.showUTC, cmd != nil)
	}
	updated, _ = enabled.Update(cmd())
	if saved := updated.(model); saved.unsaved {
		t.Fatal("successful UTC preference save remained unsaved")
	}
	config, err := loadWatchlistConfig(m.configPath, filepath.Join(t.TempDir(), "legacy.json"), nil)
	if err != nil || !config.ShowUTC {
		t.Fatalf("saved UTC preference=%t err=%v, want true", config.ShowUTC, err)
	}

	enabled.saving = true
	updated, cmd = enabled.Update(toggle)
	blocked := updated.(model)
	if !blocked.showUTC || cmd != nil || blocked.status != "Wait for the current save to finish" {
		t.Fatalf("toggle during save changed preference or scheduled save: showUTC=%t cmd=%v status=%q", blocked.showUTC, cmd != nil, blocked.status)
	}
}

func TestPickerFiltersAndAddsCatalogCity(t *testing.T) {
	m := testModel([]city{{Name: "London", Timezone: "Europe/London"}})
	m.height = 24
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	m.showUTC = true
	m.catalog = []cityRecord{
		{GeoNameID: 1, Name: "Munich", ASCIIName: "Muenchen", Country: "Germany", Timezone: "Europe/Berlin"},
		{GeoNameID: 2, Name: "Tokyo", ASCIIName: "Tokyo", Country: "Japan", Timezone: "Asia/Tokyo"},
	}

	updated, cmd := m.openPicker()
	if cmd == nil {
		t.Fatal("opening the picker did not start text cursor blinking")
	}
	picker := updated.(model)
	if got := len(picker.picker.VisibleItems()); got != len(m.catalog) {
		t.Fatalf("unfiltered picker has %d visible items, want %d", got, len(m.catalog))
	}
	picker.picker.SetFilterText("Muenchen")
	if got := len(picker.picker.VisibleItems()); got != 1 {
		t.Fatalf("filtered visible items=%d, want 1", got)
	}

	updated, cmd = picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	added := updated.(model)
	if added.screen != manageScreen || len(added.cities) != 2 {
		t.Fatalf("selected catalog entry was not added: screen=%d cities=%#v", added.screen, added.cities)
	}
	if added.cities[1].GeoNameID != 1 || added.cities[1].Timezone != "Europe/Berlin" {
		t.Fatalf("added city lost catalog identity or timezone: %#v", added.cities[1])
	}
	if cmd == nil {
		t.Fatal("adding a city did not schedule persistence")
	}
	updated, _ = added.Update(cmd())
	if updated.(model).unsaved {
		t.Fatal("successful save left city changes marked unsaved")
	}
	savedConfig, err := loadWatchlistConfig(m.configPath, filepath.Join(t.TempDir(), "legacy.json"), nil)
	if err != nil || !savedConfig.ShowUTC {
		t.Fatalf("city save lost UTC preference: showUTC=%t err=%v", savedConfig.ShowUTC, err)
	}
}

func TestPickerTypingDoesNotQuitAndCatalogIDsPreventDuplicates(t *testing.T) {
	entry := cityRecord{GeoNameID: 42, Name: "Example City", Country: "Example Country", Timezone: "UTC"}
	m := testModel([]city{{GeoNameID: 42, Name: "Example City", Country: "Example Country", Timezone: "UTC"}})
	m.height = 20
	m.catalog = []cityRecord{entry}
	updated, _ := m.openPicker()
	picker := updated.(model)
	updated, _ = picker.Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'}))
	picker = updated.(model)
	if picker.screen != pickerScreen || picker.picker.FilterValue() != "q" {
		t.Fatalf("typing q did not stay in the city filter: screen=%d filter=%q", picker.screen, picker.picker.FilterValue())
	}

	updated, cmd := picker.addCity(entry)
	if cmd != nil || len(updated.(model).cities) != 1 {
		t.Fatal("an already-selected GeoNames ID was added twice")
	}
}

func TestPickerTypingFiltersCityNamesBeforeSelection(t *testing.T) {
	m := testModel(nil)
	m.height = 24
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	m.catalog = []cityRecord{
		{GeoNameID: 1, Name: "Caen", ASCIIName: "Caen", Region: "Normandy", Country: "France", Timezone: "Europe/Paris"},
		{GeoNameID: 2, Name: "Paris", ASCIIName: "Paris", Country: "France", Timezone: "Europe/Paris"},
	}
	updated, _ := m.openPicker()
	picker := updated.(model)
	for _, character := range "Paris" {
		msg := tea.KeyPressMsg(tea.Key{Text: string(character), Code: character})
		updated, _ = picker.Update(msg)
		picker = updated.(model)
	}
	if picker.picker.FilterValue() != "Paris" {
		t.Fatalf("typed filter=%q, want Paris", picker.picker.FilterValue())
	}
	updated, _ = picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	added := updated.(model)
	if len(added.cities) != 1 || added.cities[0].GeoNameID != 2 {
		t.Fatalf("selected result after typing Paris = %#v, want Paris (GeoNames ID 2)", added.cities)
	}
}

func TestPickerAddsCurrentlyHighlightedDuplicateNameEntry(t *testing.T) {
	m := testModel(nil)
	m.height = 24
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	m.catalog = []cityRecord{
		{GeoNameID: 1, Name: "Los Angeles", ASCIIName: "Los Angeles", Region: "California", Country: "United States", Timezone: "America/Los_Angeles"},
		{GeoNameID: 2, Name: "Los Angeles", ASCIIName: "Los Angeles", Region: "Madrid", Country: "Spain", Timezone: "Europe/Madrid"},
	}
	updated, _ := m.openPicker()
	picker := updated.(model)
	picker.picker.SetFilterText("Los Angeles")
	if len(picker.picker.VisibleItems()) != 2 {
		t.Fatalf("filtered visible items=%d, want 2", len(picker.picker.VisibleItems()))
	}
	updated, _ = picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	picker = updated.(model)
	if picker.picker.SelectedItem() == nil || picker.picker.Index() != 1 {
		t.Fatalf("selected index=%d, want second filtered entry highlighted", picker.picker.Index())
	}
	updated, _ = picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	added := updated.(model)
	if len(added.cities) != 1 || added.cities[0].GeoNameID != 2 {
		t.Fatalf("selected duplicate-name entry = %#v, want GeoNames ID 2 (Madrid)", added.cities)
	}
}

func TestCityFilterPrioritizesCityNamesOverTimezoneMatches(t *testing.T) {
	targets := []string{
		cityRecord{Name: "Caen", ASCIIName: "Caen", Country: "France", Timezone: "Europe/Paris"}.FilterValue(),
		cityRecord{Name: "Paris", ASCIIName: "Paris", Country: "France", Timezone: "Europe/Paris"}.FilterValue(),
	}
	matches := cityFilter("Paris", targets)
	if len(matches) == 0 || matches[0].Index != 1 {
		t.Fatalf("city-name result ranks = %#v, want Paris at index 1 first", matches)
	}
}

func TestCityFilterSupportsCountryAndTimezonePrefixes(t *testing.T) {
	targets := []string{
		cityRecord{Name: "Berlin", Country: "Germany", CountryCode: "DE", Timezone: "Europe/Paris"}.FilterValue(),
		cityRecord{Name: "Paris", ASCIIName: "Paris", Country: "France", CountryCode: "FR", Timezone: "Europe/Berlin"}.FilterValue(),
		cityRecord{Name: "New York", Country: "United States", CountryCode: "US", Timezone: "America/New_York"}.FilterValue(),
	}
	tests := []struct {
		name string
		term string
		want []int
	}{
		{name: "country name with spaces", term: "country:United States", want: []int{2}},
		{name: "country code case insensitive", term: "CoUnTrY:us", want: []int{2}},
		{name: "timezone partial value", term: "tz:Paris", want: []int{0}},
		{name: "country does not match timezone", term: "country:Paris"},
		{name: "timezone does not match country", term: "tz:Germany"},
		{name: "empty country operand", term: "country:", want: []int{0, 1, 2}},
		{name: "unknown prefix is an ordinary query", term: "place:Berlin"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matches := cityFilter(test.term, targets)
			if len(matches) != len(test.want) {
				t.Fatalf("matches for %q = %#v, want indexes %v", test.term, matches, test.want)
			}
			for index, want := range test.want {
				if matches[index].Index != want {
					t.Errorf("match[%d].Index = %d, want %d", index, matches[index].Index, want)
				}
			}
		})
	}
}

func TestRemovingLastCityLeavesUsableEmptyState(t *testing.T) {
	m := testModel([]city{{Name: "London", Timezone: "Europe/London"}})
	m.screen = manageScreen
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	updated, cmd := m.removeCity()
	removed := updated.(model)
	if len(removed.cities) != 0 || removed.selected != 0 || cmd == nil {
		t.Fatalf("last city removal failed: cities=%#v selected=%d cmd=%v", removed.cities, removed.selected, cmd != nil)
	}
	if !strings.Contains(removed.View().Content, "No cities on your clock") {
		t.Fatal("empty management view does not offer a path to add cities")
	}
}

func TestEmbeddedCityCatalogHasUniqueIDsAndValidZones(t *testing.T) {
	contents, err := bundledData.ReadFile("data/city_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog []cityRecord
	if err := json.Unmarshal(contents, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog) < 20000 {
		t.Fatalf("catalog has only %d records", len(catalog))
	}
	ids := make(map[int64]struct{}, len(catalog))
	for _, record := range catalog {
		if record.GeoNameID <= 0 || record.Name == "" || record.Country == "" {
			t.Fatalf("incomplete catalog record: %#v", record)
		}
		if _, ok := ids[record.GeoNameID]; ok {
			t.Fatalf("duplicate GeoNames ID %d", record.GeoNameID)
		}
		ids[record.GeoNameID] = struct{}{}
		if _, err := time.LoadLocation(record.Timezone); err != nil {
			t.Fatalf("city %q has invalid timezone %q: %v", record.Name, record.Timezone, err)
		}
	}
}

func TestViewRendersCityTimesAndInvalidZones(t *testing.T) {
	m := testModel([]city{
		{Name: "London", Timezone: "Europe/London"},
		{Name: "Tokyo", Timezone: "Asia/Tokyo"},
		{Name: "New York", Timezone: "America/New_York"},
		{Name: "Broken", Timezone: "Not/AZone"},
	})

	rendered := m.View()
	if !rendered.AltScreen {
		t.Fatal("rendered view does not use the alternate screen")
	}
	view := rendered.Content
	if !strings.Contains(view, "London") {
		t.Fatal("rendered view does not contain the valid city")
	}
	for _, value := range []string{"DAY", "DATE", "TIME", "UTC OFFSET", "Fri", "02 Jan 2026", "15:04:05", "+00:00", "+09:00", "-05:00"} {
		if !strings.Contains(view, value) {
			t.Fatalf("rendered view does not contain %q", value)
		}
	}
	if !strings.Contains(view, "Broken") || !strings.Contains(view, "invalid time zone") {
		t.Fatal("rendered view does not report the invalid timezone")
	}
}

func TestViewRendersDaylightSavingOffset(t *testing.T) {
	m := testModel([]city{{Name: "New York", Timezone: "America/New_York"}})
	m.now = time.Date(2026, time.July, 2, 15, 4, 5, 0, time.UTC)

	if view := m.View().Content; !strings.Contains(view, "-04:00") {
		t.Fatalf("rendered view does not contain the summer UTC offset: %q", view)
	}
}

func TestViewRendersUTCReferenceInBothLayouts(t *testing.T) {
	m := testModel([]city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}})
	m.showUTC = true
	m.now = time.Date(2026, time.January, 3, 0, 4, 5, 0, time.UTC)

	m.width = 80
	fullView := m.View().Content
	for _, want := range []string{"UTC", "Sat", "03 Jan 2026", "00:04:05", "+00:00"} {
		if !strings.Contains(fullView, want) {
			t.Errorf("full UTC view is missing %q", want)
		}
	}
	if strings.Index(fullView, "Tokyo") > strings.LastIndex(fullView, "UTC") {
		t.Fatal("UTC reference was not rendered after the city rows")
	}

	m.width = 45
	compactView := m.View().Content
	if strings.Contains(compactView, "UTC OFFSET") || !strings.Contains(compactView, "UTC") || !strings.Contains(compactView, "00:04") {
		t.Fatalf("compact view does not render the UTC reference correctly: %q", compactView)
	}
	for _, line := range strings.Split(compactView, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("compact UTC row exceeds terminal width: %d > %d: %q", width, m.width, line)
		}
	}
}
func TestViewAlternatesRowBackgrounds(t *testing.T) {
	m := testModel([]city{
		{Name: "London", Timezone: "Europe/London"},
		{Name: "Tokyo", Timezone: "Asia/Tokyo"},
		{Name: "New York", Timezone: "America/New_York"},
	})

	view := m.View().Content
	if !strings.Contains(view, ";40m") {
		t.Fatal("rendered rows do not include the dark alternate background")
	}
	if !strings.Contains(view, ";48;5;236m") {
		t.Fatal("rendered rows do not include the gray alternate background")
	}
}

func TestUpdateHandlesQuitHelpResizeAndTick(t *testing.T) {
	m := testModel(nil)

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 48, Height: 12})
	resized := updated.(model)
	if resized.width != 48 || resized.help.Width() != 48 {
		t.Fatalf("resize was not applied: width=%d help-width=%d", resized.width, resized.help.Width())
	}

	updated, cmd = resized.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	withHelp := updated.(model)
	if !withHelp.help.ShowAll {
		t.Fatal("help toggle did not enable full help")
	}
	if cmd != nil {
		t.Fatal("help toggle returned an unexpected command")
	}

	tickTime := time.Date(2026, time.January, 2, 16, 4, 5, 0, time.UTC)
	updated, cmd = withHelp.Update(tickMsg(tickTime))
	if cmd == nil {
		t.Fatal("tick did not schedule the next tick")
	}
	if !updated.(model).now.Equal(tickTime) {
		t.Fatal("tick did not update the current time")
	}

	_, cmd = withHelp.Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'}))
	if cmd == nil {
		t.Fatal("quit did not return a command")
	}
}

func TestExpandedHelpMatchesActiveScreen(t *testing.T) {
	clockModel := testModel(nil)
	clockModel.width = 100
	clockModel.help.ShowAll = true

	clockView := clockModel.View().Content
	for _, want := range []string{"quit", "more help", "manage cities"} {
		if !strings.Contains(clockView, want) {
			t.Errorf("clock help is missing %q", want)
		}
	}
	for _, unwanted := range []string{"add city", "remove city", "move up", "retry save", "cycle sort order", "choose city"} {
		if strings.Contains(clockView, unwanted) {
			t.Errorf("clock help unexpectedly contains %q", unwanted)
		}
	}

	updated, _ := clockModel.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	manageModel := updated.(model)
	if !manageModel.help.ShowAll {
		t.Fatal("expanded help did not remain open when entering city management")
	}
	manageView := manageModel.View().Content
	for _, want := range []string{"UTC reference: off", "u toggle UTC reference"} {
		if !strings.Contains(manageView, want) {
			t.Errorf("management view is missing %q", want)
		}
	}
	for _, want := range []string{"quit", "more help", "back", "add city", "remove city", "toggle UTC reference", "move up", "retry save", "cycle sort order"} {
		if !strings.Contains(manageView, want) {
			t.Errorf("management help is missing %q", want)
		}
	}
	for _, unwanted := range []string{"manage cities", "choose city"} {
		if strings.Contains(manageView, unwanted) {
			t.Errorf("management help unexpectedly contains %q", unwanted)
		}
	}

	updated, _ = manageModel.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	clockModel = updated.(model)
	if !strings.Contains(clockModel.View().Content, "manage cities") {
		t.Fatal("clock help did not return after leaving city management")
	}
}

func TestPickerKeepsFilteringInsteadOfShowingGlobalHelp(t *testing.T) {
	manageModel := testModel(nil)
	manageModel.width = 80
	manageModel.height = 20
	manageModel.help.ShowAll = true
	manageModel.screen = manageScreen
	manageModel.catalog = []cityRecord{{Name: "Example City", ASCIIName: "Example City", Timezone: "UTC"}}

	updated, _ := manageModel.openPicker()
	pickerModel := updated.(model)
	updated, _ = pickerModel.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	pickerModel = updated.(model)

	if pickerModel.picker.FilterValue() != "?" {
		t.Fatalf("picker filter=%q, want '?'", pickerModel.picker.FilterValue())
	}
	if !pickerModel.help.ShowAll {
		t.Fatal("opening the picker changed the expanded-help state")
	}
	pickerModel.width = 40
	pickerView := pickerModel.View().Content
	if !strings.Contains(pickerView, "country:Japan") || !strings.Contains(pickerView, "tz:Paris") {
		t.Fatal("picker filter examples were not visible at a narrow width")
	}
	if strings.Contains(pickerView, "manage cities") || strings.Contains(pickerView, "more help") {
		t.Fatal("global help was rendered over the picker")
	}
}

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

func TestViewStaysWithinNarrowWidth(t *testing.T) {
	m := testModel([]city{
		{Name: "A Very Long City Name", Timezone: "Asia/Tokyo"},
		{Name: "New York", Timezone: "America/New_York"},
	})
	m.width = 36

	for _, line := range strings.Split(m.View().Content, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("rendered line exceeds terminal width: %d > %d: %q", width, m.width, line)
		}
	}

	m.width = 46
	view := m.View().Content
	if !strings.Contains(view, "UTC OFFSET") {
		t.Fatal("full-width view does not include the UTC offset column")
	}
	for _, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("rendered line exceeds terminal width at full-table boundary: %d > %d: %q", width, m.width, line)
		}
	}

	m.width = 45
	view = m.View().Content
	if strings.Contains(view, "UTC OFFSET") {
		t.Fatal("compact view unexpectedly includes the full-table header")
	}
	for _, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("rendered line exceeds terminal width at compact boundary: %d > %d: %q", width, m.width, line)
		}
	}

	m.width = 20
	for _, line := range strings.Split(m.View().Content, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("compact rendered line exceeds terminal width: %d > %d: %q", width, m.width, line)
		}
	}
}
