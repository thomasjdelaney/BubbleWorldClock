package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "time/tzdata"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

//go:embed cities.json data/city_catalog.json
var bundledData embed.FS

type city struct {
	GeoNameID   int64  `json:"geoname_id,omitempty"`
	Name        string `json:"name"`
	CountryCode string `json:"country_code,omitempty"`
	Country     string `json:"country,omitempty"`
	Admin1Code  string `json:"admin1_code,omitempty"`
	Region      string `json:"region,omitempty"`
	Timezone    string `json:"timezone"`
}

type tickMsg time.Time

type sortMode string

const (
	sortNameAscending    sortMode = "name_asc"
	sortNameDescending   sortMode = "name_desc"
	sortOffsetAscending  sortMode = "offset_asc"
	sortOffsetDescending sortMode = "offset_desc"
)

func (s sortMode) valid() bool {
	switch s {
	case sortNameAscending, sortNameDescending, sortOffsetAscending, sortOffsetDescending:
		return true
	default:
		return false
	}
}

func (s sortMode) label() string {
	switch s {
	case sortNameDescending:
		return "city name (Z-A)"
	case sortOffsetAscending:
		return "UTC offset (most negative first)"
	case sortOffsetDescending:
		return "UTC offset (most positive first)"
	default:
		return "city name (A-Z)"
	}
}

func (s sortMode) next() sortMode {
	switch s {
	case sortNameAscending:
		return sortNameDescending
	case sortNameDescending:
		return sortOffsetAscending
	case sortOffsetAscending:
		return sortOffsetDescending
	default:
		return sortNameAscending
	}
}

type screen uint8

const (
	clockScreen screen = iota
	manageScreen
	pickerScreen
)

type keyMap struct {
	Quit       key.Binding
	ToggleHelp key.Binding
	Manage     key.Binding
	AddCity    key.Binding
	RemoveCity key.Binding
	Back       key.Binding
	CursorUp   key.Binding
	CursorDown key.Binding
	ChooseCity key.Binding
	RetrySave  key.Binding
	Sort       key.Binding
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
		Manage: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "manage cities"),
		),
		AddCity: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add city"),
		),
		RemoveCity: key.NewBinding(
			key.WithKeys("d", "delete"),
			key.WithHelp("d", "remove city"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		CursorUp: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		CursorDown: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		ChooseCity: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose city"),
		),
		RetrySave: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "retry save"),
		),
		Sort: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "cycle sort order"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.ToggleHelp}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Quit, k.ToggleHelp, k.Manage},
		{k.AddCity, k.RemoveCity, k.Back},
		{k.CursorUp, k.CursorDown, k.ChooseCity, k.RetrySave, k.Sort},
	}
}

type model struct {
	cities     []city
	catalog    []cityRecord
	picker     list.Model
	screen     screen
	selected   int
	configPath string
	sortMode   sortMode
	status     string
	saving     bool
	unsaved    bool
	now        time.Time
	width      int
	height     int
	help       help.Model
	keys       keyMap
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)
		if m.screen == pickerScreen {
			m.picker.SetSize(msg.Width, max(1, msg.Height-6))
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	case tickMsg:
		m.now = time.Time(msg)
		m.sortCitiesPreservingSelection()
		return m, tick()
	case citySaveResultMsg:
		m.saving = false
		if msg.err != nil {
			m.unsaved = true
			m.status = "Save failed: " + msg.err.Error() + " (press s to retry)"
		} else {
			m.unsaved = false
			m.status = "Changes saved"
		}
		return m, nil
	}
	if m.screen == pickerScreen {
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case pickerScreen:
		if key.Matches(msg, m.keys.Back) {
			m.screen = manageScreen
			m.status = ""
			return m, nil
		}
		if key.Matches(msg, m.keys.ChooseCity) {
			m.picker.SetFilterText(m.picker.FilterValue())
			selected, ok := m.picker.SelectedItem().(cityRecord)
			if !ok {
				m.picker.SetFilterState(list.Filtering)
				m.status = "No matching city"
				return m, nil
			}
			return m.addCity(selected)
		}
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd

	case manageScreen:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Back):
			m.screen = clockScreen
		case key.Matches(msg, m.keys.AddCity):
			return m.openPicker()
		case key.Matches(msg, m.keys.RemoveCity):
			return m.removeCity()
		case key.Matches(msg, m.keys.CursorUp):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.CursorDown):
			if m.selected < len(m.cities)-1 {
				m.selected++
			}
		case key.Matches(msg, m.keys.RetrySave) && m.unsaved && !m.saving:
			return m, m.startSave()
		case key.Matches(msg, m.keys.Sort):
			m.sortMode = m.sortMode.next()
			m.sortCitiesPreservingSelection()
			return m, m.startSave()
		}
		return m, nil

	default:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Manage):
			m.screen = manageScreen
			m.status = ""
		}
		return m, nil
	}
}

func (m model) openPicker() (tea.Model, tea.Cmd) {
	items := make([]list.Item, len(m.catalog))
	for i, record := range m.catalog {
		items[i] = record
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	m.picker = list.New(items, delegate, max(1, m.width), max(1, m.height-6))
	m.picker.Filter = cityFilter
	m.picker.Title = "Add a city"
	m.picker.SetShowHelp(false)
	m.picker.SetStatusBarItemName("city", "cities")
	m.picker.DisableQuitKeybindings()
	m.picker.SetFilterText("")
	m.picker.SetFilterState(list.Filtering)
	m.screen = pickerScreen
	m.status = ""
	return m, textinput.Blink
}

func (m model) addCity(record cityRecord) (tea.Model, tea.Cmd) {
	if m.saving {
		m.status = "Wait for the current save to finish"
		return m, nil
	}
	if _, err := time.LoadLocation(record.Timezone); err != nil {
		m.status = "Catalog timezone is invalid: " + record.Timezone
		return m, nil
	}
	newCity := city{
		GeoNameID:   record.GeoNameID,
		Name:        record.Name,
		CountryCode: record.CountryCode,
		Country:     record.Country,
		Admin1Code:  record.Admin1Code,
		Region:      record.Region,
		Timezone:    record.Timezone,
	}
	for _, existing := range m.cities {
		if sameCity(existing, newCity) {
			m.screen = manageScreen
			m.status = "That city is already on the clock"
			return m, nil
		}
	}
	m.cities = append(m.cities, newCity)
	m.selected = len(m.cities) - 1
	m.sortCitiesPreservingSelection()
	m.screen = manageScreen
	return m, m.startSave()
}

func sameCity(first, second city) bool {
	if first.GeoNameID != 0 && second.GeoNameID != 0 {
		return first.GeoNameID == second.GeoNameID
	}
	return strings.EqualFold(first.Name, second.Name) && first.Timezone == second.Timezone
}

func (m model) removeCity() (tea.Model, tea.Cmd) {
	if m.saving {
		m.status = "Wait for the current save to finish"
		return m, nil
	}
	if len(m.cities) == 0 {
		m.status = "No cities to remove"
		return m, nil
	}
	m.cities = append(m.cities[:m.selected], m.cities[m.selected+1:]...)
	if len(m.cities) > 0 {
		m.selected = min(m.selected, len(m.cities)-1)
	} else {
		m.selected = 0
	}
	if m.selected >= len(m.cities) {
		m.selected = max(0, len(m.cities)-1)
	}
	return m, m.startSave()
}

func (m *model) startSave() tea.Cmd {
	m.saving = true
	m.unsaved = true
	m.status = "Saving changes..."
	return saveCitiesCmd(m.configPath, m.cities, m.sortMode)
}

func cityIdentity(value city) string {
	if value.GeoNameID != 0 {
		return fmt.Sprintf("id:%d", value.GeoNameID)
	}
	return "city:" + strings.ToLower(value.Name) + "\x00" + value.Timezone
}

func (m *model) sortCitiesPreservingSelection() {
	if !m.sortMode.valid() {
		m.sortMode = sortNameAscending
	}
	var selectedIdentity string
	if m.selected >= 0 && m.selected < len(m.cities) {
		selectedIdentity = cityIdentity(m.cities[m.selected])
	}
	sort.SliceStable(m.cities, func(first, second int) bool {
		left, right := m.cities[first], m.cities[second]
		leftLocation, leftErr := time.LoadLocation(left.Timezone)
		rightLocation, rightErr := time.LoadLocation(right.Timezone)
		leftOffset, rightOffset := 0, 0
		if leftErr == nil {
			_, leftOffset = m.now.In(leftLocation).Zone()
		}
		if rightErr == nil {
			_, rightOffset = m.now.In(rightLocation).Zone()
		}
		if m.sortMode == sortOffsetAscending || m.sortMode == sortOffsetDescending {
			if leftErr != rightErr {
				return leftErr == nil
			}
			if leftOffset != rightOffset {
				if m.sortMode == sortOffsetDescending {
					return leftOffset > rightOffset
				}
				return leftOffset < rightOffset
			}
		}
		leftName := strings.ToLower(left.Name)
		rightName := strings.ToLower(right.Name)
		if leftName != rightName {
			if m.sortMode == sortNameDescending {
				return leftName > rightName
			}
			return leftName < rightName
		}
		if left.Timezone != right.Timezone {
			return left.Timezone < right.Timezone
		}
		return left.GeoNameID < right.GeoNameID
	})
	if selectedIdentity == "" {
		m.selected = min(m.selected, max(0, len(m.cities)-1))
		return
	}
	for index, value := range m.cities {
		if cityIdentity(value) == selectedIdentity {
			m.selected = index
			return
		}
	}
	m.selected = min(m.selected, max(0, len(m.cities)-1))
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
	if m.width > 0 {
		m.help.SetWidth(m.width)
	}
	var content string
	switch m.screen {
	case manageScreen:
		content = m.viewManage()
	case pickerScreen:
		content = m.picker.View()
		if m.status != "" {
			content += "\n\n" + m.status
		}
		content += "\n\n" + truncate("type to filter | enter add | esc back", max(1, m.width))
	default:
		content = m.viewClock()
	}
	if helpView := m.help.View(m.keys); helpView != "" && m.screen != pickerScreen {
		content += "\n\n" + helpView
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m model) viewClock() string {
	const (
		dayFormat    = "Mon"
		dateFormat   = "02 Jan 2006"
		timeFormat   = "15:04:05"
		offsetFormat = "-07:00"
	)

	width := m.width
	if width == 0 {
		width = 80
	}
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	offsetWidth := lipgloss.Width("UTC OFFSET")
	compact := contentWidth < 44
	dayWidth := lipgloss.Width(dayFormat)
	dateWidth := lipgloss.Width(dateFormat)
	timeWidth := lipgloss.Width(timeFormat)

	nameWidth := 4
	for i := range m.cities {
		if cityWidth := lipgloss.Width(m.cityLabel(i)); cityWidth > nameWidth {
			nameWidth = cityWidth
		}
	}
	maxNameWidth := contentWidth - dayWidth - dateWidth - timeWidth - offsetWidth - 8
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
			headerStyle.Render(fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s", nameWidth, "CITY", dayWidth, "DAY", dateWidth, "DATE", timeWidth, "TIME", offsetWidth, "UTC OFFSET")),
			borderStyle.Render(strings.Repeat("─", contentWidth)),
		)
	}
	if len(m.cities) == 0 {
		lines = append(lines, mutedStyle.Render("No cities yet. Press m to manage cities."))
	}
	for i, city := range m.cities {
		label := m.cityLabel(i)
		location, err := time.LoadLocation(city.Timezone)
		if err != nil {
			if compact {
				row := withRowBackground(mutedStyle, i).Render(truncate(label+" invalid time zone", contentWidth))
				lines = append(lines, row)
				continue
			}
			name := truncate(label, nameWidth)
			row := withRowBackground(cityStyle, i).Render(fmt.Sprintf("%-*s  ", nameWidth, name)) + withRowBackground(mutedStyle, i).Render("invalid time zone")
			lines = append(lines, row)
			continue
		}

		if compact {
			value := m.now.In(location).Format("15:04")
			row := truncate(label, contentWidth-lipgloss.Width(value)-1) + " " + value
			lines = append(lines, withRowBackground(timeStyle, i).Render(truncate(row, contentWidth)))
			continue
		}
		name := truncate(label, nameWidth)
		localTime := m.now.In(location)
		row := withRowBackground(cityStyle, i).Render(fmt.Sprintf("%-*s", nameWidth, name)) + "  " +
			withRowBackground(timeStyle, i).Render(fmt.Sprintf("%-*s  %-*s  %-*s  %-*s", dayWidth, localTime.Format(dayFormat), dateWidth, localTime.Format(dateFormat), timeWidth, localTime.Format(timeFormat), offsetWidth, localTime.Format(offsetFormat)))
		lines = append(lines, row)
	}
	lines = append(lines, mutedStyle.Render(truncate("m manage cities", contentWidth)))

	return strings.Join(lines, "\n")
}

func (m model) viewManage() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	contentWidth := max(1, width-2)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("24"))
	lines := []string{
		titleStyle.Render("Manage cities"),
		mutedStyle.Render(strings.Repeat("─", contentWidth)),
	}
	if len(m.cities) == 0 {
		lines = append(lines, "No cities on your clock. Press a to search the city catalog.")
	} else {
		for i, city := range m.cities {
			marker := "  "
			style := lipgloss.NewStyle()
			if i == m.selected {
				marker = "> "
				style = selectedStyle
			}
			line := marker + m.cityLabel(i) + "  " + city.Timezone
			lines = append(lines, style.Render(truncate(line, contentWidth)))
		}
	}
	if m.status != "" {
		lines = append(lines, "", mutedStyle.Render(truncate(m.status, contentWidth)))
	}
	lines = append(lines, "", mutedStyle.Render(truncate("sort: "+m.sortMode.label(), contentWidth)))
	lines = append(lines, mutedStyle.Render(truncate("up/down select | o cycle sort order | a add | d remove | esc back", contentWidth)))
	return strings.Join(lines, "\n")
}

func (m model) cityLabel(index int) string {
	if index < 0 || index >= len(m.cities) {
		return ""
	}
	city := m.cities[index]
	duplicates := 0
	for _, candidate := range m.cities {
		if candidate.Name == city.Name {
			duplicates++
		}
	}
	if duplicates < 2 {
		return city.Name
	}
	context := strings.Join(nonEmpty([]string{city.Region, city.Country}), ", ")
	if context == "" {
		context = city.Timezone
	}
	return city.Name + " (" + context + ")"
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
	defaultData, err := bundledData.ReadFile("cities.json")
	if err != nil {
		fmt.Println("Error loading default cities:", err)
		os.Exit(1)
	}
	var defaults []city
	if err := json.Unmarshal(defaultData, &defaults); err != nil {
		fmt.Println("Error decoding default cities:", err)
		os.Exit(1)
	}
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		fmt.Println("Error locating user config directory:", err)
		os.Exit(1)
	}
	configPath := filepath.Join(configDirectory, "BubbleWorldClock", "cities.json")
	watchlist, err := loadWatchlistConfig(configPath, "cities.json", defaults)
	if err != nil {
		fmt.Println("Error loading city settings:", err)
		os.Exit(1)
	}
	catalogData, err := bundledData.ReadFile("data/city_catalog.json")
	if err != nil {
		fmt.Println("Error loading city catalog:", err)
		os.Exit(1)
	}
	var catalog []cityRecord
	if err := json.Unmarshal(catalogData, &catalog); err != nil {
		fmt.Println("Error decoding city catalog:", err)
		os.Exit(1)
	}

	app := model{
		cities:     watchlist.Cities,
		catalog:    catalog,
		configPath: configPath,
		sortMode:   watchlist.Sort,
		now:        time.Now(),
		help:       help.New(),
		keys:       newKeyMap(),
	}
	app.sortCitiesPreservingSelection()
	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg {
		return tickMsg(now)
	})
}
