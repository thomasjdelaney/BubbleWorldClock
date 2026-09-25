package main

import (
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

func TestViewRendersCityTimesAndInvalidZones(t *testing.T) {
	m := testModel([]city{
		{Name: "London", Timezone: "Europe/London"},
		{Name: "Broken", Timezone: "Not/AZone"},
	})

	view := m.View().Content
	if !strings.Contains(view, "London") {
		t.Fatal("rendered view does not contain the valid city")
	}
	for _, value := range []string{"DAY", "DATE", "TIME", "Fri", "02 Jan 2026", "15:04:05"} {
		if !strings.Contains(view, value) {
			t.Fatalf("rendered view does not contain %q", value)
		}
	}
	if !strings.Contains(view, "Broken") || !strings.Contains(view, "invalid time zone") {
		t.Fatal("rendered view does not report the invalid timezone")
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

	m.width = 20
	for _, line := range strings.Split(m.View().Content, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("compact rendered line exceeds terminal width: %d > %d: %q", width, m.width, line)
		}
	}
}
