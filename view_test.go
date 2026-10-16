package main

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
)

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
					position = strings.LastIndex(view, "\x1b[1;38;5;214")
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

func TestClockViewUsesConfiguredColumnOrderAndKeepsCityFirst(t *testing.T) {
	m := testModel([]city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}})
	m.width = 120
	m.columns = []columnPreference{
		{ID: columnDaylight, Visible: true},
		{ID: columnTime, Visible: false},
		{ID: columnDate, Visible: true},
		{ID: columnUTCOffset, Visible: true},
		{ID: columnDay, Visible: true},
	}
	m.showUTC = true
	view := m.View().Content
	header := strings.Split(view, "\n")[2]
	cityIndex := strings.Index(header, "CITY")
	daylightIndex := strings.Index(header, "DAYLIGHT")
	dateIndex := strings.Index(header, "DATE")
	utcOffsetIndex := strings.Index(header, "UTC OFFSET")
	dayIndex := strings.LastIndex(header, "DAY")
	if !(cityIndex >= 0 && cityIndex < daylightIndex && daylightIndex < dateIndex && dateIndex < utcOffsetIndex && utcOffsetIndex < dayIndex) {
		t.Fatalf("header does not follow City plus configured order: %q", header)
	}
	if strings.Count(header, "TIME") != 1 || !strings.Contains(header, "TIME ZONE") {
		t.Fatalf("hidden Time column is still rendered or Time zone is missing: %q", header)
	}
	if !strings.Contains(view, "UTC") || !strings.Contains(view, "02 Jan 2026") {
		t.Fatalf("UTC row did not use selected columns: %q", view)
	}
}

func TestClockViewCanHideEveryOptionalColumn(t *testing.T) {
	m := testModel([]city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}})
	m.columns = defaultColumnPreferences()
	for index := range m.columns {
		m.columns[index].Visible = false
	}
	header := strings.Split(m.View().Content, "\n")[2]
	if !strings.Contains(header, "CITY") {
		t.Fatalf("City column disappeared when optional columns were hidden: %q", header)
	}
	for _, hidden := range []string{"DAY", "DATE", "TIME", "UTC OFFSET", "TIME ZONE", "DAYLIGHT"} {
		if strings.Contains(header, hidden) {
			t.Errorf("hidden column %q still appears in header %q", hidden, header)
		}
	}
}

func TestClockViewTimezoneColumnUsesConfiguredNamingStyle(t *testing.T) {
	for _, test := range []struct {
		name  string
		now   time.Time
		style timezoneNameStyle
		want  string
	}{
		{name: "IANA identifier", now: time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC), style: timezoneNameIANA, want: "Europe/Berlin"},
		{name: "winter abbreviation", now: time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC), style: timezoneNameAbbreviation, want: "CET"},
		{name: "summer abbreviation", now: time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC), style: timezoneNameAbbreviation, want: "CEST"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := testModel([]city{{Name: "Berlin", Timezone: "Europe/Berlin"}})
			m.now = test.now
			m.timezoneNameStyle = test.style
			if _, err := time.LoadLocation(m.cities[0].Timezone); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(m.View().Content, test.want) {
				t.Fatalf("clock view does not contain timezone value %q: %q", test.want, m.View().Content)
			}
		})
	}
}

func TestTimezoneDisplayReplacesUnderscoresAndSizesToDisplayedValues(t *testing.T) {
	zoneID := "America/North_Dakota/Center"
	localTime := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)
	if got := timezoneNameValue(zoneID, localTime, timezoneNameIANA); got != "America/North Dakota/Center" {
		t.Fatalf("timezoneNameValue() = %q, want underscores replaced", got)
	}
	cities := []city{{Name: "Center", Timezone: zoneID}}
	if got, want := timezoneColumnWidth(cities, localTime, timezoneNameIANA, false), len("America/North Dakota/Center"); got != want {
		t.Fatalf("IANA timezone column width = %d, want longest value width %d", got, want)
	}
	if got, want := timezoneColumnWidth(cities, localTime, timezoneNameAbbreviation, false), len("TIME ZONE"); got != want {
		t.Fatalf("abbreviation timezone column width = %d, want header width %d", got, want)
	}
}

func TestDefaultTimezoneColumnPrecedesDaylight(t *testing.T) {
	preferences := defaultColumnPreferences()
	if len(preferences) < 2 || preferences[len(preferences)-2].ID != columnTimezone || preferences[len(preferences)-1].ID != columnDaylight {
		t.Fatalf("default columns do not end with timezone then daylight: %#v", preferences)
	}
}

func TestClockUsesSelectedDateAndTimeLayoutsInCompactView(t *testing.T) {
	m := testModel([]city{{Name: "Berlin", Timezone: "Europe/Berlin"}})
	m.now = time.Date(2026, time.January, 2, 12, 4, 5, 0, time.UTC)
	m.dateFormat = "2006-01-02"
	m.timeFormat = "3:04 PM"
	m.showUTC = true
	m.width = 65
	columns := visibleClockColumns(m.columns)
	_, _, compact := clockLayoutWidths(m.width-2, 6, columns)
	if !compact {
		t.Fatal("test setup did not produce a compact layout")
	}
	view := m.View().Content
	if !strings.Contains(view, "2026-01-02") || !strings.Contains(view, "1:04 PM") || !strings.Contains(view, "12:04 PM") {
		t.Fatalf("clock view did not preserve the selected formats in compact layout: %q", view)
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
	if !strings.Contains(view, "Broken") || !strings.Contains(view, "invalid") {
		t.Fatal("rendered view does not report the invalid timezone")
	}
}

func TestCityClockStaysCompactAndLeftAlignedOnWideTerminal(t *testing.T) {
	m := testModel([]city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}})
	m.width = 160

	view := m.View().Content
	lines := strings.Split(view, "\n")
	var dividerWidths []int
	for _, line := range lines {
		width := lipgloss.Width(line)
		if strings.Contains(line, "─") {
			dividerWidths = append(dividerWidths, width)
		}
	}
	if len(dividerWidths) != 2 {
		t.Fatalf("got %d table dividers, want 2: %q", len(dividerWidths), view)
	}
	if dividerWidths[0] != dividerWidths[1] {
		t.Fatalf("table dividers have different widths: %v", dividerWidths)
	}
	if dividerWidths[0] >= m.width-2 {
		t.Fatalf("table divider still stretches toward terminal width: %d cells", dividerWidths[0])
	}
}

func TestClockLayoutDoesNotExpandCityColumnIntoSpareWidth(t *testing.T) {
	columns := visibleClockColumns(defaultColumnPreferences())
	widths, gap, compact := clockLayoutWidths(160, 13, columns)
	if compact {
		t.Fatal("wide layout was unexpectedly compact")
	}
	if widths[0] != 13 {
		t.Fatalf("city column expanded to %d cells, want its 13-cell natural width", widths[0])
	}
	daylightIndex := len(widths) - 1
	if widths[daylightIndex] != 48 {
		t.Fatalf("daylight column width = %d, want its 48-cell maximum", widths[daylightIndex])
	}
	total := gap * (len(widths) - 1)
	for _, width := range widths {
		total += width
	}
	if total >= 160 {
		t.Fatalf("natural table width %d consumes the 160-cell terminal", total)
	}
}

func TestCityClockUsesWidestCityNameWithoutTruncatingIt(t *testing.T) {
	widestName := "A Remarkably Long City Name"
	m := testModel([]city{
		{Name: "Paris", Timezone: "Europe/Paris"},
		{Name: widestName, Timezone: "Asia/Tokyo"},
	})
	m.width = 160

	view := m.View().Content
	if !strings.Contains(view, widestName) {
		t.Fatalf("clock view truncated the widest city name %q: %q", widestName, view)
	}
	lines := strings.Split(view, "\n")
	var dividerWidth int
	for _, line := range lines {
		if strings.Contains(line, "─") {
			dividerWidth = lipgloss.Width(line)
			break
		}
	}
	if dividerWidth < lipgloss.Width(widestName) {
		t.Fatalf("table width %d is narrower than widest city name %d", dividerWidth, lipgloss.Width(widestName))
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

	m.width = 120
	view := m.View().Content
	if !strings.Contains(view, "UTC OFFSET") {
		t.Fatal("full-width view does not include the UTC offset column")
	}
	for _, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("rendered line exceeds terminal width: %d > %d: %q", width, m.width, line)
		}
	}

	m.width = 56
	view = m.View().Content
	if !strings.Contains(view, "A Very Long City Name") {
		t.Fatal("compact view truncated the widest city name")
	}
	for _, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("rendered line exceeds terminal width at compact boundary: %d > %d: %q", width, m.width, line)
		}
	}

	m.width = 55
	view = m.View().Content
	if !strings.Contains(view, "A Very Long City Name") {
		t.Fatal("narrow view truncated the widest city name")
	}
	for _, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("rendered line exceeds terminal width at compact boundary: %d > %d: %q", width, m.width, line)
		}
	}

	m.width = 20
	view = m.View().Content
	if !strings.Contains(view, "A Very Long City Name") {
		t.Fatalf("narrow view truncated the city name: %q", view)
	}
}
