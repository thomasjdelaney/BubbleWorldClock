package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type city struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type tickMsg time.Time

type keyMap struct {
	Quit       key.Binding
	ToggleHelp key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		ToggleHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "more help"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.ToggleHelp}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Quit, k.ToggleHelp}}
}

type model struct {
	cities []city
	now    time.Time
	width  int
	help   help.Model
	keys   keyMap
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.help.SetWidth(msg.Width)
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
	case tickMsg:
		m.now = time.Time(msg)
		return m, tick()
	}

	return m, nil
}

func rowBackgroundStyle(index int) lipgloss.Style {
	if index%2 == 0 {
		return lipgloss.NewStyle().Background(lipgloss.Color("0"))
	}
	return lipgloss.NewStyle().Background(lipgloss.Color("236"))
}

func withRowBackground(style lipgloss.Style, index int) lipgloss.Style {
	bg := rowBackgroundStyle(index)
	return style.Background(bg.GetBackground())
}

func (m model) View() tea.View {
	const (
		dayFormat  = "Mon"
		dateFormat = "02 Jan 2006"
		timeFormat = "15:04:05"
	)

	width := m.width
	if width == 0 {
		width = 80
	}
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	compact := contentWidth < 34
	dayWidth := lipgloss.Width(dayFormat)
	dateWidth := lipgloss.Width(dateFormat)
	timeWidth := lipgloss.Width(timeFormat)

	nameWidth := 4
	for _, city := range m.cities {
		if cityWidth := lipgloss.Width(city.Name); cityWidth > nameWidth {
			nameWidth = cityWidth
		}
	}
	maxNameWidth := contentWidth - dayWidth - dateWidth - timeWidth - 6
	if maxNameWidth < 1 {
		maxNameWidth = 1
	}
	if nameWidth > maxNameWidth {
		nameWidth = maxNameWidth
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
	cityStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	title := truncate("Bubble World Clock", contentWidth)
	lines := []string{titleStyle.Render(title)}
	if !compact {
		lines = append(lines,
			mutedStyle.Render(strings.Repeat("─", contentWidth)),
			headerStyle.Render(fmt.Sprintf("%-*s  %-*s  %-*s  %-*s", nameWidth, "CITY", dayWidth, "DAY", dateWidth, "DATE", timeWidth, "TIME")),
			borderStyle.Render(strings.Repeat("─", contentWidth)),
		)
	}
	for i, city := range m.cities {
		location, err := time.LoadLocation(city.Timezone)
		if err != nil {
			if compact {
				row := withRowBackground(mutedStyle, i).Render(truncate(city.Name+" invalid time zone", contentWidth))
				lines = append(lines, row)
				continue
			}
			name := truncate(city.Name, nameWidth)
			row := withRowBackground(cityStyle, i).Render(fmt.Sprintf("%-*s  ", nameWidth, name)) + withRowBackground(mutedStyle, i).Render("invalid time zone")
			lines = append(lines, row)
			continue
		}

		if compact {
			value := m.now.In(location).Format("15:04")
			row := truncate(city.Name, contentWidth-lipgloss.Width(value)-1) + " " + value
			lines = append(lines, withRowBackground(timeStyle, i).Render(truncate(row, contentWidth)))
			continue
		}
		name := truncate(city.Name, nameWidth)
		localTime := m.now.In(location)
		row := withRowBackground(cityStyle, i).Render(fmt.Sprintf("%-*s", nameWidth, name)) + "  " +
			withRowBackground(timeStyle, i).Render(fmt.Sprintf("%-*s  %-*s  %-*s", dayWidth, localTime.Format(dayFormat), dateWidth, localTime.Format(dateFormat), timeWidth, localTime.Format(timeFormat)))
		lines = append(lines, row)
	}

	helpView := m.help.View(m.keys)
	if helpView != "" {
		lines = append(lines, "", helpView)
	}

	return tea.NewView(strings.Join(lines, "\n"))
}

func truncate(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	return string([]rune(value)[:width-1]) + "…"
}

func main() {
	cities, err := loadCities("cities.json")
	if err != nil {
		fmt.Println("Error loading cities:", err)
		os.Exit(1)
	}

	if _, err := tea.NewProgram(model{
		cities: cities,
		now:    time.Now(),
		help:   help.New(),
		keys:   newKeyMap(),
	}).Run(); err != nil {
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
