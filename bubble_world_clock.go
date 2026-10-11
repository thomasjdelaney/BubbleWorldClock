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
	columnsScreen
	formatScreen
)

type keyMap struct {
	Quit                 key.Binding
	ToggleHelp           key.Binding
	Manage               key.Binding
	AddCity              key.Binding
	RemoveCity           key.Binding
	Back                 key.Binding
	CursorUp             key.Binding
	CursorDown           key.Binding
	ChooseCity           key.Binding
	EditFormat           key.Binding
	RetrySave            key.Binding
	Sort                 key.Binding
	ToggleUTC            key.Binding
	ManageColumns        key.Binding
	ManageFormats        key.Binding
	ToggleColumn         key.Binding
	MoveColumnLeft       key.Binding
	MoveColumnRight      key.Binding
	PreviousFormatOption key.Binding
	NextFormatOption     key.Binding
	ResetColumns         key.Binding
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
		EditFormat: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit custom format"),
		),
		RetrySave: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "retry save"),
		),
		Sort: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "cycle sort order"),
		),
		ToggleUTC: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "toggle UTC reference"),
		),
		ManageColumns: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "manage columns"),
		),
		ManageFormats: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "manage formats"),
		),
		ToggleColumn: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("space", "toggle visibility"),
		),
		MoveColumnLeft: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "move left"),
		),
		MoveColumnRight: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "move right"),
		),
		PreviousFormatOption: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←/→", "change option"),
		),
		NextFormatOption: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("←/→", "change option"),
		),
		ResetColumns: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reset columns"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.ToggleHelp}
}

type screenHelpKeyMap struct {
	keyMap
	screen screen
}

func (k screenHelpKeyMap) FullHelp() [][]key.Binding {
	switch k.screen {
	case manageScreen:
		return [][]key.Binding{
			{k.Quit, k.ToggleHelp, k.Back},
			{k.AddCity, k.RemoveCity, k.ToggleUTC, k.ManageColumns, k.ManageFormats},
			{k.CursorUp, k.CursorDown, k.RetrySave, k.Sort},
		}
	case columnsScreen:
		return [][]key.Binding{
			{k.Quit, k.ToggleHelp, k.Back},
			{k.CursorUp, k.CursorDown, k.ToggleColumn, k.MoveColumnLeft, k.MoveColumnRight, k.Manage, k.ManageFormats},
			{k.ResetColumns, k.RetrySave},
		}
	case formatScreen:
		return [][]key.Binding{
			{k.Quit, k.ToggleHelp, k.Back},
			{k.CursorUp, k.CursorDown, k.PreviousFormatOption, k.EditFormat, k.Manage, k.ManageColumns},
			{k.RetrySave},
		}
	default:
		return [][]key.Binding{{k.Quit, k.ToggleHelp, k.Manage, k.ManageColumns, k.ManageFormats}}
	}
}

type model struct {
	cities               []city
	catalog              []cityRecord
	picker               list.Model
	screen               screen
	selected             int
	configPath           string
	sortMode             sortMode
	showUTC              bool
	columns              []columnPreference
	timezoneNameStyle    timezoneNameStyle
	dateFormat           string
	timeFormat           string
	selectedColumn       int
	selectedFormat       formatSetting
	selectedFormatOption int
	formatEditing        bool
	formatInput          textinput.Model
	status               string
	saving               bool
	unsaved              bool
	now                  time.Time
	width                int
	height               int
	help                 help.Model
	keys                 keyMap
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
		if m.formatEditing {
			m.formatInput.SetWidth(max(1, msg.Width-8))
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
	if m.screen == formatScreen && m.formatEditing {
		var cmd tea.Cmd
		m.formatInput, cmd = m.formatInput.Update(msg)
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
			selectedBefore, hadSelection := m.picker.SelectedItem().(cityRecord)
			m.picker.SetFilterText(m.picker.FilterValue())
			selected, ok := m.picker.SelectedItem().(cityRecord)
			if hadSelection && ok && cityRecordIdentity(selectedBefore) != cityRecordIdentity(selected) {
				for _, item := range m.picker.VisibleItems() {
					candidate, okCandidate := item.(cityRecord)
					if okCandidate && cityRecordIdentity(candidate) == cityRecordIdentity(selectedBefore) {
						selected = candidate
						break
					}
				}
			}
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
		case key.Matches(msg, m.keys.ManageColumns):
			m = m.openColumns()
		case key.Matches(msg, m.keys.ManageFormats):
			m = m.openFormats()
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
		case key.Matches(msg, m.keys.ToggleUTC):
			if m.saving {
				m.status = "Wait for the current save to finish"
				return m, nil
			}
			m.showUTC = !m.showUTC
			return m, m.startSave()
		}
		return m, nil

	case columnsScreen:
		return m.updateColumnKey(msg)
	case formatScreen:
		return m.updateFormatKey(msg)

	default:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Manage):
			m.screen = manageScreen
			m.status = ""
		case key.Matches(msg, m.keys.ManageColumns):
			m = m.openColumns()
		case key.Matches(msg, m.keys.ManageFormats):
			m = m.openFormats()
		}
		return m, nil
	}
}

func (m model) openColumns() model {
	m.screen = columnsScreen
	m.columns = normalizeColumnPreferences(m.columns)
	m.selectedColumn = 0
	m.status = ""
	return m
}

func (m model) openFormats() model {
	m.screen = formatScreen
	m.selectedFormat = dateFormatSetting
	m.selectedFormatOption = formatOptionIndex(normalizeDateLayout(m.dateFormat), dateFormatOptions)
	m.status = ""
	return m
}

func (m model) currentFormatLayout(setting formatSetting) string {
	switch setting {
	case dateFormatSetting:
		return normalizeDateLayout(m.dateFormat)
	case timeFormatSetting:
		return normalizeTimeLayout(m.timeFormat)
	default:
		return ""
	}
}

func (m *model) setFormatLayout(setting formatSetting, layout string) {
	if setting == dateFormatSetting {
		m.dateFormat = layout
	} else if setting == timeFormatSetting {
		m.timeFormat = layout
	}
}

func (m model) updateFormatKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.formatEditing {
		switch {
		case key.Matches(msg, m.keys.Back):
			m.formatEditing = false
			m.formatInput.Blur()
			m.status = ""
			return m, nil
		case key.Matches(msg, m.keys.EditFormat):
			if m.saving {
				m.status = "Wait for the current save to finish"
				return m, nil
			}
			layout := strings.TrimSpace(m.formatInput.Value())
			if layout == "" {
				m.status = "Format cannot be empty"
				return m, nil
			}
			m.setFormatLayout(m.selectedFormat, m.formatInput.Value())
			m.formatEditing = false
			m.formatInput.Blur()
			m.status = ""
			return m, m.startSave()
		default:
			var cmd tea.Cmd
			m.formatInput, cmd = m.formatInput.Update(msg)
			return m, cmd
		}
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.ToggleHelp):
		m.help.ShowAll = !m.help.ShowAll
	case key.Matches(msg, m.keys.Back):
		m.screen = clockScreen
		m.status = ""
	case key.Matches(msg, m.keys.Manage):
		m.screen = manageScreen
		m.status = ""
	case key.Matches(msg, m.keys.ManageColumns):
		m = m.openColumns()
	case key.Matches(msg, m.keys.RetrySave) && m.unsaved && !m.saving:
		return m, m.startSave()
	case key.Matches(msg, m.keys.CursorUp):
		if m.selectedFormat > dateFormatSetting {
			m.selectedFormat--
			m.selectedFormatOption = m.currentFormatOptionIndex(m.selectedFormat)
		}
	case key.Matches(msg, m.keys.CursorDown):
		if m.selectedFormat < timezoneFormatSetting {
			m.selectedFormat++
			m.selectedFormatOption = m.currentFormatOptionIndex(m.selectedFormat)
		}
	case key.Matches(msg, m.keys.PreviousFormatOption), key.Matches(msg, m.keys.NextFormatOption):
		if m.saving {
			m.status = "Wait for the current save to finish"
			return m, nil
		}
		options := formatOptionsFor(m.selectedFormat)
		direction := -1
		if key.Matches(msg, m.keys.NextFormatOption) {
			direction = 1
		}
		m.selectedFormatOption = (m.selectedFormatOption + direction + len(options)) % len(options)
		if m.selectedFormat == timezoneFormatSetting {
			if m.selectedFormatOption == 0 {
				m.timezoneNameStyle = timezoneNameIANA
			} else {
				m.timezoneNameStyle = timezoneNameAbbreviation
			}
			return m, m.startSave()
		}
		if layout := options[m.selectedFormatOption].Layout; layout != "" {
			m.setFormatLayout(m.selectedFormat, layout)
			return m, m.startSave()
		}
	case key.Matches(msg, m.keys.EditFormat):
		if m.selectedFormat == dateFormatSetting || m.selectedFormat == timeFormatSetting {
			options := formatOptionsFor(m.selectedFormat)
			if m.selectedFormatOption == len(options)-1 {
				if m.saving {
					m.status = "Wait for the current save to finish"
					return m, nil
				}
				m.formatInput = textinput.New()
				m.formatInput.Prompt = "Go layout: "
				m.formatInput.CharLimit = 128
				m.formatInput.SetWidth(max(20, m.width-8))
				m.formatInput.SetValue(m.currentFormatLayout(m.selectedFormat))
				cmd := m.formatInput.Focus()
				m.formatEditing = true
				m.status = ""
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m model) currentFormatOptionIndex(setting formatSetting) int {
	if setting == timezoneFormatSetting {
		if m.timezoneNameStyle == timezoneNameAbbreviation {
			return 1
		}
		return 0
	}
	return formatOptionIndex(m.currentFormatLayout(setting), formatOptionsFor(setting))
}

func (m model) updateColumnKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.ToggleHelp):
		m.help.ShowAll = !m.help.ShowAll
	case key.Matches(msg, m.keys.Back):
		m.screen = clockScreen
		m.status = ""
	case key.Matches(msg, m.keys.Manage):
		m.screen = manageScreen
		m.status = ""
	case key.Matches(msg, m.keys.ManageFormats):
		m = m.openFormats()
	case key.Matches(msg, m.keys.CursorUp):
		if m.selectedColumn > 0 {
			m.selectedColumn--
		}
	case key.Matches(msg, m.keys.CursorDown):
		if m.selectedColumn < len(m.columns)-1 {
			m.selectedColumn++
		}
	case key.Matches(msg, m.keys.RetrySave) && m.unsaved && !m.saving:
		return m, m.startSave()
	case key.Matches(msg, m.keys.ResetColumns):
		if m.saving {
			m.status = "Wait for the current save to finish"
			return m, nil
		}
		m.columns = defaultColumnPreferences()
		m.selectedColumn = 0
		return m, m.startSave()
	case key.Matches(msg, m.keys.ToggleColumn):
		if m.saving {
			m.status = "Wait for the current save to finish"
			return m, nil
		}
		if m.selectedColumn >= 0 && m.selectedColumn < len(m.columns) {
			m.columns[m.selectedColumn].Visible = !m.columns[m.selectedColumn].Visible
			return m, m.startSave()
		}
	case key.Matches(msg, m.keys.MoveColumnLeft), key.Matches(msg, m.keys.MoveColumnRight):
		if m.saving {
			m.status = "Wait for the current save to finish"
			return m, nil
		}
		direction := -1
		if key.Matches(msg, m.keys.MoveColumnRight) {
			direction = 1
		}
		previousSelection := m.selectedColumn
		m.columns, m.selectedColumn = moveColumnPreference(m.columns, m.selectedColumn, direction)
		if previousSelection == m.selectedColumn {
			return m, nil
		}
		return m, m.startSave()
	}
	return m, nil
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

func cityRecordIdentity(value cityRecord) string {
	if value.GeoNameID != 0 {
		return fmt.Sprintf("id:%d", value.GeoNameID)
	}
	return "city:" + strings.ToLower(value.Name) + "\x00" + strings.ToLower(value.Country) + "\x00" + value.Timezone
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
	return saveCitiesCmd(m.configPath, m.cities, m.sortMode, m.showUTC, m.columns, m.timezoneNameStyle, m.dateFormat, m.timeFormat)
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
		return cityComesBefore(m.cities[first], m.cities[second], m.sortMode, m.now)
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

func cityComesBefore(left, right city, mode sortMode, now time.Time) bool {
	if !mode.valid() {
		mode = sortNameAscending
	}
	leftLocation, leftErr := time.LoadLocation(left.Timezone)
	rightLocation, rightErr := time.LoadLocation(right.Timezone)
	leftOffset, rightOffset := 0, 0
	if leftErr == nil {
		_, leftOffset = now.In(leftLocation).Zone()
	}
	if rightErr == nil {
		_, rightOffset = now.In(rightLocation).Zone()
	}
	if mode == sortOffsetAscending || mode == sortOffsetDescending {
		if leftErr != rightErr {
			return leftErr == nil
		}
		if leftOffset != rightOffset {
			if mode == sortOffsetDescending {
				return leftOffset > rightOffset
			}
			return leftOffset < rightOffset
		}
	}
	leftName := strings.ToLower(left.Name)
	rightName := strings.ToLower(right.Name)
	if leftName != rightName {
		if mode == sortNameDescending {
			return leftName > rightName
		}
		return leftName < rightName
	}
	if left.Timezone != right.Timezone {
		return left.Timezone < right.Timezone
	}
	return left.GeoNameID < right.GeoNameID
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "update":
			os.Exit(runUpdate())
		case "version":
			fmt.Println(currentVersion())
			return
		case "--help", "-h", "help":
			fmt.Println("Usage: bubble-world-clock [update|version]")
			return
		}
	}
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
		cities:            watchlist.Cities,
		catalog:           catalog,
		configPath:        configPath,
		sortMode:          watchlist.Sort,
		showUTC:           watchlist.ShowUTC,
		columns:           watchlist.Columns,
		timezoneNameStyle: watchlist.TimezoneNameStyle,
		dateFormat:        watchlist.DateFormat,
		timeFormat:        watchlist.TimeFormat,
		now:               time.Now(),
		help:              help.New(),
		keys:              newKeyMap(),
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
