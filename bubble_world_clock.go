package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	daylightScreen
)

type pickerPurpose uint8

const (
	addCityPicker pickerPurpose = iota
	daylightTimezonePicker
)

const (
	daylightModeSetting = iota
	daylightReferenceSetting
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
	DaylightSettings     key.Binding
	ChangeDaylightZone   key.Binding
	CycleDaylightMode    key.Binding
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
		DaylightSettings: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "daylight settings"),
		),
		ChangeDaylightZone: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "change time zone"),
		),
		CycleDaylightMode: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←/→", "change timeline mode"),
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
			{k.CursorUp, k.CursorDown, k.ToggleColumn, k.MoveColumnLeft, k.MoveColumnRight, k.DaylightSettings, k.Manage, k.ManageFormats},
			{k.ResetColumns, k.RetrySave},
		}
	case formatScreen:
		return [][]key.Binding{
			{k.Quit, k.ToggleHelp, k.Back},
			{k.CursorUp, k.CursorDown, k.PreviousFormatOption, k.EditFormat, k.Manage, k.ManageColumns},
			{k.RetrySave},
		}
	case daylightScreen:
		return [][]key.Binding{
			{k.Quit, k.ToggleHelp, k.Back},
			{k.CursorUp, k.CursorDown, k.CycleDaylightMode, k.ChangeDaylightZone},
			{k.RetrySave},
		}
	default:
		return [][]key.Binding{{k.Quit, k.ToggleHelp, k.Manage, k.ManageColumns, k.ManageFormats}}
	}
}

type model struct {
	cities                  []city
	catalog                 []cityRecord
	picker                  list.Model
	pickerPurpose           pickerPurpose
	pickerReturnScreen      screen
	screen                  screen
	selected                int
	configPath              string
	sortMode                sortMode
	showUTC                 bool
	daylightTimezone        string
	daylightCityName        string
	daylightMode            daylightMode
	selectedDaylightSetting int
	columns                 []columnPreference
	timezoneNameStyle       timezoneNameStyle
	dateFormat              string
	timeFormat              string
	selectedColumn          int
	selectedFormat          formatSetting
	selectedFormatOption    int
	formatEditing           bool
	formatInput             textinput.Model
	status                  string
	saving                  bool
	unsaved                 bool
	now                     time.Time
	width                   int
	height                  int
	help                    help.Model
	keys                    keyMap
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
			m.screen = m.pickerReturnScreen
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
			if m.pickerPurpose == daylightTimezonePicker {
				return m.setDaylightReference(selected)
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
	case daylightScreen:
		return m.updateDaylightKey(msg)

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

func (m *model) startSave() tea.Cmd {
	m.saving = true
	m.unsaved = true
	m.status = "Saving changes..."
	return saveWatchlistCmd(m.configPath, watchlistConfig{
		Cities:            m.cities,
		Sort:              m.sortMode,
		ShowUTC:           m.showUTC,
		DaylightTimezone:  m.daylightTimezone,
		DaylightCityName:  m.daylightCityName,
		DaylightMode:      m.daylightMode,
		Columns:           m.columns,
		TimezoneNameStyle: m.timezoneNameStyle,
		DateFormat:        m.dateFormat,
		TimeFormat:        m.timeFormat,
	})
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
		daylightTimezone:  watchlist.DaylightTimezone,
		daylightCityName:  watchlist.DaylightCityName,
		daylightMode:      watchlist.DaylightMode,
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
