package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

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
