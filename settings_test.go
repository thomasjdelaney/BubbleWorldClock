package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func TestColumnPreferencesNormalizeUnknownDuplicateAndMissingEntries(t *testing.T) {
	got := normalizeColumnPreferences([]columnPreference{
		{ID: columnDaylight, Visible: false},
		{ID: "unknown", Visible: true},
		{ID: columnDaylight, Visible: true},
		{ID: columnTime, Visible: false},
	})
	want := []columnPreference{
		{ID: columnTimezone, Visible: true},
		{ID: columnDaylight, Visible: false},
		{ID: columnTime, Visible: false},
		{ID: columnDay, Visible: true},
		{ID: columnDate, Visible: true},
		{ID: columnUTCOffset, Visible: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized preferences=%#v, want %#v", got, want)
	}
}

func TestManageColumnsToggleReorderResetAndPersist(t *testing.T) {
	m := testModel([]city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}})
	m.screen = manageScreen
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))
	current := updated.(model)
	if current.screen != columnsScreen || !reflect.DeepEqual(current.columns, defaultColumnPreferences()) {
		t.Fatalf("opening columns screen: screen=%d columns=%#v", current.screen, current.columns)
	}

	updated, cmd := current.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	current = updated.(model)
	if current.columns[0].Visible || cmd == nil {
		t.Fatal("Space did not hide the selected Day column and schedule a save")
	}
	updated, _ = current.Update(cmd())
	current = updated.(model)
	loaded, err := loadWatchlistConfig(m.configPath, filepath.Join(t.TempDir(), "legacy.json"), nil)
	if err != nil || loaded.Columns[0].Visible {
		t.Fatalf("hidden column did not persist: columns=%#v err=%v", loaded.Columns, err)
	}

	updated, cmd = current.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	current = updated.(model)
	if current.columns[0].ID != columnDate || current.columns[1].ID != columnDay || cmd == nil {
		t.Fatalf("right arrow did not reorder selected column: %#v", current.columns)
	}
	updated, _ = current.Update(cmd())
	current = updated.(model)
	loaded, err = loadWatchlistConfig(m.configPath, filepath.Join(t.TempDir(), "legacy.json"), nil)
	if err != nil || loaded.Columns[0].ID != columnDate || loaded.Columns[1].ID != columnDay {
		t.Fatalf("reordered columns did not persist: columns=%#v err=%v", loaded.Columns, err)
	}
	updated, cmd = current.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	current = updated.(model)
	if !reflect.DeepEqual(current.columns, defaultColumnPreferences()) || cmd == nil {
		t.Fatal("r did not restore default columns and schedule a save")
	}
	updated, _ = current.Update(cmd())
	loaded, err = loadWatchlistConfig(m.configPath, filepath.Join(t.TempDir(), "legacy.json"), nil)
	if err != nil || !reflect.DeepEqual(loaded.Columns, defaultColumnPreferences()) {
		t.Fatalf("default column preferences did not persist: columns=%#v err=%v", loaded.Columns, err)
	}
}

func TestManageColumnsScreenShowsFixedCityAndControls(t *testing.T) {
	m := testModel(nil)
	m.screen = manageScreen
	m.help.ShowAll = true
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))
	columns := updated.(model)
	view := columns.View().Content
	for _, want := range []string{"Manage columns", "[x] City (fixed first)", "Time zone", "Daylight", "space", "toggle visibility", "move left", "move right", "reset columns"} {
		if !strings.Contains(view, want) {
			t.Errorf("column manager is missing %q", want)
		}
	}
}

func TestFormatScreenTimezoneNameStylePersists(t *testing.T) {
	m := testModel([]city{{Name: "Berlin", Timezone: "Europe/Berlin"}})
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	m.columns = defaultColumnPreferences()
	m.screen = formatScreen
	m.selectedFormat = timezoneFormatSetting
	m.selectedFormatOption = 0

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	current := updated.(model)
	if current.timezoneNameStyle != timezoneNameAbbreviation || cmd == nil {
		t.Fatalf("timezone style selection: style=%q cmd=%v", current.timezoneNameStyle, cmd != nil)
	}
	updated, _ = current.Update(cmd())
	current = updated.(model)
	loaded, err := readWatchlist(m.configPath)
	if err != nil || loaded.TimezoneNameStyle != timezoneNameAbbreviation {
		t.Fatalf("timezone naming style did not persist: style=%q err=%v", loaded.TimezoneNameStyle, err)
	}
}

func TestFormatScreenShowsPresetExamplesAndTimezonePreviews(t *testing.T) {
	m := testModel([]city{{Name: "Berlin", Timezone: "Europe/Berlin"}})
	m.screen = formatScreen
	m.selectedFormat = dateFormatSetting
	m.selectedFormatOption = 0
	view := m.View().Content
	for _, want := range []string{
		"Date format", "02 Jan 2006", "2006-01-02", "02/01/2006", "01/02/2006",
		"Time format", "15:04:05", "15:04", "3:04:05 PM", "3:04 PM",
		"Timezone name", "Europe/Berlin", "CET / CEST", "Date: Mon/Monday", "enter edit Custom",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("format screen is missing %q", want)
		}
	}
	m.selectedFormat = timeFormatSetting
	if !strings.Contains(m.View().Content, "Time: 15=24h") {
		t.Fatal("format screen does not show time layout token guidance")
	}
}

func TestFormatScreenSelectsAndPersistsPresetsAndCustomLayouts(t *testing.T) {
	m := testModel([]city{{Name: "Berlin", Timezone: "Europe/Berlin"}})
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	m.screen = formatScreen
	m.selectedFormat = dateFormatSetting
	m.selectedFormatOption = 0

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	current := updated.(model)
	if current.dateFormat != "2006-01-02" || cmd == nil {
		t.Fatalf("selecting ISO date format: layout=%q cmd=%v", current.dateFormat, cmd != nil)
	}
	updated, _ = current.Update(cmd())
	current = updated.(model)
	loaded, err := readWatchlist(m.configPath)
	if err != nil || loaded.DateFormat != "2006-01-02" {
		t.Fatalf("preset date format did not persist: layout=%q err=%v", loaded.DateFormat, err)
	}

	current.selectedFormatOption = len(dateFormatOptions) - 1
	updated, cmd = current.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	current = updated.(model)
	if !current.formatEditing || cmd == nil {
		t.Fatal("Enter on Custom did not start the custom layout editor")
	}
	current.formatInput.SetValue("Monday, 02 Jan")
	updated, cmd = current.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	current = updated.(model)
	if current.formatEditing || current.dateFormat != "Monday, 02 Jan" || cmd == nil {
		t.Fatalf("custom date format was not accepted: editing=%t layout=%q cmd=%v", current.formatEditing, current.dateFormat, cmd != nil)
	}
	updated, _ = current.Update(cmd())
	loaded, err = readWatchlist(m.configPath)
	if err != nil || loaded.DateFormat != "Monday, 02 Jan" || loaded.TimeFormat != defaultTimeLayout {
		t.Fatalf("custom format or defaults were not persisted: config=%#v err=%v", loaded, err)
	}
}

func TestFormatEditorRejectsEmptyAndEscapeCancels(t *testing.T) {
	m := testModel(nil)
	m.screen = formatScreen
	m.formatEditing = true
	m.selectedFormat = timeFormatSetting
	m.formatInput = textinput.New()
	m.formatInput.Focus()
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	current := updated.(model)
	if !current.formatEditing || current.timeFormat != "" || cmd != nil || current.status != "Format cannot be empty" {
		t.Fatal("empty custom format was not rejected")
	}
	updated, _ = current.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	current = updated.(model)
	if current.formatEditing || current.screen != formatScreen {
		t.Fatal("Escape did not cancel custom format editing")
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

func TestDaylightManagerSelectsTimezoneWithoutAddingCity(t *testing.T) {
	m := testModel([]city{{Name: "London", Timezone: "Europe/London"}})
	m.height = 24
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	m.columns = defaultColumnPreferences()
	m.screen = columnsScreen
	// Keep a non-DAYLIGHT column selected; the shortcut opens settings globally.
	m.selectedColumn = 0
	m.catalog = []cityRecord{
		{GeoNameID: 1, Name: "Munich", Country: "Germany", Timezone: "Europe/Berlin"},
		{GeoNameID: 2, Name: "Tokyo", Country: "Japan", Timezone: "Asia/Tokyo"},
	}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	settings := updated.(model)
	if settings.screen != daylightScreen {
		t.Fatalf("d on DAYLIGHT opened screen %d, want Daylight settings", settings.screen)
	}
	updated, _ = settings.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))
	picker := updated.(model)
	if picker.screen != pickerScreen || picker.pickerPurpose != daylightTimezonePicker {
		t.Fatalf("change time zone opened picker with purpose %d on screen %d", picker.pickerPurpose, picker.screen)
	}
	picker.picker.SetFilterText("Munich")
	updated, cmd := picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	selected := updated.(model)
	if selected.screen != daylightScreen || selected.daylightTimezone != "Europe/Berlin" || selected.daylightCityName != "Munich" {
		t.Fatalf("daylight reference selection = screen %d, %q (%q)", selected.screen, selected.daylightTimezone, selected.daylightCityName)
	}
	if len(selected.cities) != 1 || selected.cities[0].Name != "London" || cmd == nil {
		t.Fatalf("selection changed clock cities or skipped saving: cities=%#v cmd=%v", selected.cities, cmd != nil)
	}
	updated, _ = selected.Update(cmd())
	if saved := updated.(model); saved.unsaved {
		t.Fatal("saved daylight reference remained unsaved")
	}
	config, err := readWatchlist(m.configPath)
	if err != nil || config.DaylightTimezone != "Europe/Berlin" || config.DaylightCityName != "Munich" {
		t.Fatalf("saved daylight reference = %q (%q), err=%v", config.DaylightTimezone, config.DaylightCityName, err)
	}
}

func TestDaylightPickerCancelReturnsToSettings(t *testing.T) {
	m := testModel(nil)
	m.height = 20
	m.catalog = []cityRecord{{Name: "Paris", Timezone: "Europe/Paris"}}
	updated, _ := m.openDaylightSettings().openDaylightPicker()
	picker := updated.(model)
	updated, _ = picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	returned := updated.(model)
	if returned.screen != daylightScreen || returned.daylightTimezone != "" {
		t.Fatalf("cancel returned screen=%d timezone=%q, want settings and no change", returned.screen, returned.daylightTimezone)
	}
}

func TestDaylightModeSwitchPersistsAndSettingsRowNavigation(t *testing.T) {
	m := testModel([]city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}})
	m.configPath = filepath.Join(t.TempDir(), "cities.json")
	settings := m.openDaylightSettings()
	if settings.selectedDaylightSetting != daylightModeSetting {
		t.Fatalf("initial Daylight setting row=%d, want Timeline mode", settings.selectedDaylightSetting)
	}
	settingsView := settings.View().Content
	for _, want := range []string{"Timeline mode: Static daylight, moving now marker", "Now marker: UTC (UTC)", "←/→ mode"} {
		if !strings.Contains(settingsView, want) {
			t.Errorf("Daylight settings view is missing %q", want)
		}
	}
	updated, cmd := settings.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	changed := updated.(model)
	if changed.daylightMode != daylightModeRolling || cmd == nil {
		t.Fatalf("right did not select rolling mode: mode=%q save=%v", changed.daylightMode, cmd != nil)
	}
	updated, _ = changed.Update(cmd())
	if saved := updated.(model); saved.unsaved {
		t.Fatal("saved daylight mode remained unsaved")
	}
	config, err := readWatchlist(m.configPath)
	if err != nil || config.DaylightMode != daylightModeRolling {
		t.Fatalf("saved mode=%q, err=%v; want rolling", config.DaylightMode, err)
	}
	updated, _ = changed.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	referenceRow := updated.(model)
	if referenceRow.selectedDaylightSetting != daylightReferenceSetting {
		t.Fatalf("down selected row %d, want reference city", referenceRow.selectedDaylightSetting)
	}
	updated, cmd = referenceRow.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	if got := updated.(model); got.daylightMode != daylightModeRolling || cmd != nil {
		t.Fatalf("left on reference row changed mode or saved: mode=%q cmd=%v", got.daylightMode, cmd != nil)
	}
}

func TestExpandedHelpMatchesActiveScreen(t *testing.T) {
	clockModel := testModel(nil)
	clockModel.width = 100
	clockModel.help.ShowAll = true

	clockView := clockModel.View().Content
	for _, want := range []string{"quit", "more help", "manage cities", "manage columns", "manage formats"} {
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
	updated, _ = clockModel.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))
	columnsModel := updated.(model)
	if columnsModel.screen != columnsScreen {
		t.Fatal("c did not open column management from the clock screen")
	}
	if !columnsModel.help.ShowAll {
		t.Fatal("expanded help did not remain open when entering column management")
	}
	updated, _ = columnsModel.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if returned := updated.(model); returned.screen != clockScreen {
		t.Fatalf("Escape from column management returned to %q, want clock", returned.screen)
	}

	updated, _ = manageModel.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))
	columnsModel = updated.(model)
	if columnsModel.screen != columnsScreen {
		t.Fatal("c did not open column management from city management")
	}
	updated, _ = columnsModel.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if returned := updated.(model); returned.screen != clockScreen {
		t.Fatalf("Escape from column management opened from city management returned to %q, want clock", returned.screen)
	}
	manageView := manageModel.View().Content
	for _, want := range []string{"UTC reference: off", "u toggle UTC reference", "c columns"} {
		if !strings.Contains(manageView, want) {
			t.Errorf("management view is missing %q", want)
		}
	}
	for _, want := range []string{"quit", "more help", "back", "add city", "remove city", "manage columns", "manage formats", "toggle UTC reference", "move up", "retry save", "cycle sort order"} {
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

func TestFormatScreenShortcutWorksFromClockAndCityManagement(t *testing.T) {
	m := testModel(nil)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "f", Code: 'f'}))
	if got := updated.(model).screen; got != formatScreen {
		t.Fatalf("f from clock opened screen %d, want format screen", got)
	}
	m.screen = manageScreen
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "f", Code: 'f'}))
	if got := updated.(model).screen; got != formatScreen {
		t.Fatalf("f from city management opened screen %d, want format screen", got)
	}
	m.screen = columnsScreen
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "f", Code: 'f'}))
	if got := updated.(model).screen; got != formatScreen {
		t.Fatalf("f from columns opened screen %d, want format screen", got)
	}
	m.screen = formatScreen
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))
	if got := updated.(model).screen; got != columnsScreen {
		t.Fatalf("c from formats opened screen %d, want columns screen", got)
	}
	updated, _ = updated.(model).Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	if got := updated.(model).screen; got != manageScreen {
		t.Fatalf("m from columns opened screen %d, want city management", got)
	}
	m.screen = formatScreen
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	if got := updated.(model).screen; got != manageScreen {
		t.Fatalf("m from formats opened screen %d, want city management", got)
	}
}
