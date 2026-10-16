package main

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

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
