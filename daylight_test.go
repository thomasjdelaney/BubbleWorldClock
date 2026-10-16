package main

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
)

func TestViewRendersDaylightSavingOffset(t *testing.T) {
	m := testModel([]city{{Name: "New York", Timezone: "America/New_York"}})
	m.now = time.Date(2026, time.July, 2, 15, 4, 5, 0, time.UTC)

	if view := m.View().Content; !strings.Contains(view, "-04:00") {
		t.Fatalf("rendered view does not contain the summer UTC offset: %q", view)
	}
}

func TestCityDaylightIntervalsIncludeCivilTwilightAndDay(t *testing.T) {
	start := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	location := time.UTC
	value := city{Name: "Null Island", Timezone: "UTC"}
	catalog := []cityRecord{{Name: value.Name, Timezone: value.Timezone, Latitude: 0, Longitude: 0, CoordinatesKnown: true}}
	intervals, ok := cityDaylightIntervals(value, catalog, location, start)
	if !ok {
		t.Fatal("cityDaylightIntervals() did not recognize catalog coordinates")
	}

	var dayInterval lightInterval
	hasDay, hasTwilight := false, false
	for _, interval := range intervals {
		if interval.start.Before(start) || interval.end.After(start.Add(24*time.Hour)) {
			t.Errorf("interval escaped UTC day: %+v", interval)
		}
		switch interval.level {
		case lightDay:
			dayInterval, hasDay = interval, true
		case lightTwilight:
			hasTwilight = true
		}
	}
	if !hasDay || !hasTwilight {
		t.Fatalf("expected daylight and civil twilight intervals, got %+v", intervals)
	}
	if dayInterval.start.Hour() < 5 || dayInterval.start.Hour() > 7 || dayInterval.end.Hour() < 17 || dayInterval.end.Hour() > 19 {
		t.Errorf("equinox daylight interval is unexpectedly placed: %+v", dayInterval)
	}
}

func TestCityDaylightIntervalsUseLocalDatesAndReturnUTCIntervals(t *testing.T) {
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	location, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Fatal(err)
	}
	value := city{Name: "Kiritimati", Timezone: location.String()}
	catalog := []cityRecord{{Name: value.Name, Timezone: value.Timezone, Latitude: 1.8721, Longitude: -157.4278, CoordinatesKnown: true}}
	intervals, ok := cityDaylightIntervals(value, catalog, location, start)
	if !ok {
		t.Fatal("cityDaylightIntervals() did not recognize catalog coordinates")
	}
	for _, interval := range intervals {
		if interval.start.Before(start) || interval.end.After(start.Add(24*time.Hour)) {
			t.Errorf("interval escaped UTC day: %+v", interval)
		}
	}
	if got := lightAt(start.Add(20*time.Hour), intervals); got != lightDay {
		t.Errorf("lightAt(20:00 UTC) = %v, want daylight", got)
	}
}

func TestCityDaylightIntervalsHandleDaylightSavingDate(t *testing.T) {
	start := time.Date(2026, time.March, 8, 0, 0, 0, 0, time.UTC)
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	value := city{Name: "New York", Timezone: location.String()}
	catalog := []cityRecord{{Name: value.Name, Timezone: value.Timezone, Latitude: 40.7128, Longitude: -74.006, CoordinatesKnown: true}}
	intervals, ok := cityDaylightIntervals(value, catalog, location, start)
	if !ok {
		t.Fatal("cityDaylightIntervals() did not recognize catalog coordinates")
	}
	if got := lightAt(start.Add(12*time.Hour), intervals); got != lightDay {
		t.Errorf("lightAt(12:00 UTC on DST start date) = %v, want daylight", got)
	}
}

func TestDaylightWindowUsesReferenceLocalCalendarDayAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{name: "spring forward", now: time.Date(2026, time.March, 8, 12, 0, 0, 0, location), want: 23 * time.Hour},
		{name: "fall back", now: time.Date(2026, time.November, 1, 12, 0, 0, 0, location), want: 25 * time.Hour},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			start, end := daylightWindow(check.now, location)
			if got := end.Sub(start); got != check.want {
				t.Fatalf("daylight window length = %s, want %s", got, check.want)
			}
			startDate := start.In(location)
			endDate := end.In(location)
			if startDate.Hour() != 0 || endDate.Hour() != 0 || startDate.Day() == endDate.Day() {
				t.Fatalf("window does not run between local midnights: %s to %s", startDate, endDate)
			}
		})
	}
}

func TestRollingDaylightWindowIsCenteredAndExactly24Hours(t *testing.T) {
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.January, 2, 1, 30, 0, 0, location)
	start, end := daylightWindowForMode(now, location, daylightModeRolling)
	if got := end.Sub(start); got != 24*time.Hour {
		t.Fatalf("rolling window = %s, want 24h", got)
	}
	if !start.Equal(now.Add(-12*time.Hour)) || !end.Equal(now.Add(12*time.Hour)) {
		t.Fatalf("rolling window %s to %s is not centered on now %s", start, end, now)
	}
	if start.In(location).Day() == now.In(location).Day() {
		t.Fatalf("test window did not cross local midnight: start=%s now=%s", start.In(location), now.In(location))
	}
	for _, width := range []int{1, 24, 48, 49} {
		if got := daylightMarkerIndexForMode(now, start, end, width, daylightModeRolling); got != width/2 {
			t.Errorf("rolling marker at width %d = %d, want center cell %d", width, got, width/2)
		}
	}
}

func TestCityDaylightIntervalsHandlePolarDayAndNight(t *testing.T) {
	location, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		t.Fatal(err)
	}
	value := city{Name: "Longyearbyen", Timezone: location.String()}
	catalog := []cityRecord{{Name: value.Name, Timezone: value.Timezone, Latitude: 78.2232, Longitude: 15.6469, CoordinatesKnown: true}}
	checks := []struct {
		date time.Time
		want lightLevel
	}{
		{time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC), lightDay},
		{time.Date(2026, time.December, 21, 0, 0, 0, 0, time.UTC), lightNight},
	}
	for _, check := range checks {
		intervals, ok := cityDaylightIntervals(value, catalog, location, check.date)
		if !ok {
			t.Fatal("cityDaylightIntervals() did not recognize catalog coordinates")
		}
		if got := lightAt(check.date.Add(12*time.Hour), intervals); got != check.want {
			t.Errorf("lightAt(%s) = %v, want %v", check.date, got, check.want)
		}
	}
}

func TestCityClockShowsDaylightBarInBothLayouts(t *testing.T) {
	m := testModel([]city{{Name: "Tokyo", Timezone: "Asia/Tokyo"}})
	m.now = time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
	m.catalog = []cityRecord{{Name: "Tokyo", Timezone: "Asia/Tokyo", Latitude: 35.6762, Longitude: 139.6503, CoordinatesKnown: true}}

	full := m.View().Content
	for _, want := range []string{"DAYLIGHT", "UTC", "twilight", "daylight", "now (UTC)", "│"} {
		if !strings.Contains(full, want) {
			t.Errorf("full view is missing %q: %q", want, full)
		}
	}
	m.daylightTimezone = "Asia/Tokyo"
	m.daylightCityName = "Tokyo"
	if view := m.View().Content; !strings.Contains(view, "now (Tokyo)") {
		t.Fatalf("legend does not identify the selected daylight reference city: %q", view)
	}
	m.daylightTimezone = ""
	m.daylightCityName = ""
	if !strings.Contains(full, "\x1b[38;5;117;40mTokyo") {
		t.Fatal("full layout changed the city-name color")
	}
	for _, unwanted := range []string{"TIME OF DAY", "🌙", "🌅", "🌇"} {
		if strings.Contains(full, unwanted) {
			t.Errorf("full view still includes the old icon feature %q", unwanted)
		}
	}

	m.width = 45
	compact := m.View().Content
	if !strings.Contains(compact, "Tokyo") || !strings.Contains(compact, "│") {
		t.Fatalf("compact view is missing Tokyo's daylight timeline: %q", compact)
	}
	if !strings.Contains(compact, "\x1b[38;5;252;40mTokyo") {
		t.Fatal("compact layout changed the city-name color")
	}
	for _, line := range strings.Split(compact, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Errorf("compact line exceeds terminal width: %d > %d: %q", width, m.width, line)
		}
	}
}

func TestCityWithoutCatalogCoordinatesShowsUnavailableBar(t *testing.T) {
	value := city{Name: "Custom", Timezone: "Europe/Paris"}
	if _, _, ok := cityCoordinates(value, []cityRecord{{Name: "Custom", Timezone: "Europe/Paris"}}); ok {
		t.Fatal("city without coordinates unexpectedly has a location")
	}
	bar := renderDaylightBar(nil, false, 14, time.Time{}, time.Time{}, time.Time{}, 0, nil, lipgloss.NewStyle(), lipgloss.NewStyle())
	if !strings.Contains(bar, "no coordinates") || lipgloss.Width(bar) != 14 {
		t.Fatalf("missing-coordinate bar = %q, want a 14-cell unavailable label", bar)
	}
}

func TestDaylightMarkerUsesSameUTCPositionForEveryCity(t *testing.T) {
	start := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	now := start.Add(6 * time.Hour)
	if got := daylightMarkerIndex(now, start, end, 48); got != 12 {
		t.Fatalf("daylightMarkerIndex() = %d, want 12", got)
	}
	if got := daylightMarkerIndex(now, start, end, 24); got != 6 {
		t.Fatalf("daylightMarkerIndex() at compact width = %d, want 6", got)
	}
}

func TestRenderDaylightBarShowsNightTwilightDayAndCurrentMarker(t *testing.T) {
	start := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	intervals := []lightInterval{
		{start.Add(5 * time.Hour), start.Add(6 * time.Hour), lightTwilight},
		{start.Add(6 * time.Hour), start.Add(18 * time.Hour), lightDay},
		{start.Add(18 * time.Hour), start.Add(19 * time.Hour), lightTwilight},
	}
	styles := map[lightLevel]lipgloss.Style{
		lightNight:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		lightTwilight: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		lightDay:      lipgloss.NewStyle().Foreground(lipgloss.Color("229")),
	}
	bar := renderDaylightBar(intervals, true, 24, start, end, start.Add(12*time.Hour), 0, styles, lipgloss.NewStyle(), lipgloss.NewStyle())
	for _, want := range []string{"·", "*", "▒", "━", "│"} {
		if !strings.Contains(bar, want) {
			t.Errorf("daylight bar is missing %q: %q", want, bar)
		}
	}
	if width := lipgloss.Width(bar); width != 24 {
		t.Errorf("daylight bar width = %d, want 24", width)
	}
}

func TestCityCoordinatesMatchCommonCityNameVariant(t *testing.T) {
	value := city{Name: "New York", Timezone: "America/New_York"}
	lat, lon, ok := cityCoordinates(value, []cityRecord{{Name: "New York City", Timezone: "America/New_York", Latitude: 40.71427, Longitude: -74.00597, CoordinatesKnown: true}})
	if !ok || lat != 40.71427 || lon != -74.00597 {
		t.Fatalf("cityCoordinates() = (%v, %v, %v), want New York City coordinates", lat, lon, ok)
	}
}
