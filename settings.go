package main

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

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
	case key.Matches(msg, m.keys.DaylightSettings):
		m = m.openDaylightSettings()
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

func (m model) openDaylightSettings() model {
	m.screen = daylightScreen
	m.selectedDaylightSetting = daylightModeSetting
	m.status = ""
	return m
}

func (m model) updateDaylightKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.ToggleHelp):
		m.help.ShowAll = !m.help.ShowAll
	case key.Matches(msg, m.keys.Back):
		m.screen = columnsScreen
		m.status = ""
	case key.Matches(msg, m.keys.ChangeDaylightZone):
		if m.saving {
			m.status = "Wait for the current save to finish"
			return m, nil
		}
		return m.openDaylightPicker()
	case key.Matches(msg, m.keys.CursorUp):
		if m.selectedDaylightSetting > daylightModeSetting {
			m.selectedDaylightSetting--
		}
	case key.Matches(msg, m.keys.CursorDown):
		if m.selectedDaylightSetting < daylightReferenceSetting {
			m.selectedDaylightSetting++
		}
	case key.Matches(msg, m.keys.CycleDaylightMode):
		if m.selectedDaylightSetting != daylightModeSetting {
			return m, nil
		}
		if m.saving {
			m.status = "Wait for the current save to finish"
			return m, nil
		}
		if normalizeDaylightMode(m.daylightMode) == daylightModeCalendar {
			m.daylightMode = daylightModeRolling
		} else {
			m.daylightMode = daylightModeCalendar
		}
		return m, m.startSave()
	case key.Matches(msg, m.keys.RetrySave) && m.unsaved && !m.saving:
		return m, m.startSave()
	}
	return m, nil
}

func (m model) setDaylightReference(record cityRecord) (tea.Model, tea.Cmd) {
	if m.saving {
		m.screen = daylightScreen
		m.status = "Wait for the current save to finish"
		return m, nil
	}
	if _, err := time.LoadLocation(record.Timezone); err != nil {
		m.screen = daylightScreen
		m.status = "Catalog timezone is invalid: " + record.Timezone
		return m, nil
	}
	m.daylightTimezone = record.Timezone
	m.daylightCityName = record.Name
	m.screen = daylightScreen
	return m, m.startSave()
}
