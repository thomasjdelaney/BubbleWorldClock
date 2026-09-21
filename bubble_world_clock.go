package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
)

type city struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type tickMsg time.Time

type model struct {
	cities []city
	now    time.Time
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key := msg.String(); key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
	case tickMsg:
		m.now = time.Time(msg)
		return m, tick()
	}

	return m, nil
}

func (m model) View() tea.View {
	view := "Bubble World Clock\n\n"
	for _, city := range m.cities {
		location, err := time.LoadLocation(city.Timezone)
		if err != nil {
			view += fmt.Sprintf("%s: invalid time zone\n", city.Name)
			continue
		}

		view += fmt.Sprintf("%-12s %s\n", city.Name, m.now.In(location).Format("Mon 02 Jan 2006 15:04:05"))
	}
	view += "\nPress q to quit.\n"
	return tea.NewView(view)
}

func main() {
	cities, err := loadCities("cities.json")
	if err != nil {
		fmt.Println("Error loading cities:", err)
		os.Exit(1)
	}

	if _, err := tea.NewProgram(model{cities: cities, now: time.Now()}).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func loadCities(filename string) ([]city, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cities []city
	if err := json.Unmarshal(contents, &cities); err != nil {
		return nil, err
	}

	return cities, nil
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg {
		return tickMsg(now)
	})
}
