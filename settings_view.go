package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

func (m model) viewDaylightSettings() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	contentWidth := max(1, width-2)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	lines := []string{
		titleStyle.Render("Daylight settings"),
		mutedStyle.Render(strings.Repeat("─", contentWidth)),
	}
	modeMarker := "  "
	if m.selectedDaylightSetting == daylightModeSetting {
		modeMarker = "> "
	}
	modeStyle := lipgloss.NewStyle()
	if m.selectedDaylightSetting == daylightModeSetting {
		modeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("24"))
	}
	lines = append(lines, modeStyle.Render(truncate(modeMarker+"Timeline mode: "+normalizeDaylightMode(m.daylightMode).label(), contentWidth)))
	zoneMarker := "  "
	if m.selectedDaylightSetting == daylightReferenceSetting {
		zoneMarker = "> "
	}
	zoneStyle := lipgloss.NewStyle()
	if m.selectedDaylightSetting == daylightReferenceSetting {
		zoneStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("24"))
	}
	lines = append(lines, zoneStyle.Render(truncate(zoneMarker+"Now marker: "+m.daylightLegendCityName()+" ("+normalizeDaylightTimezone(m.daylightTimezone)+")", contentWidth)))
	if normalizeDaylightMode(m.daylightMode) == daylightModeRolling {
		lines = append(lines, mutedStyle.Render("Rolling 24-hour window centered on now."))
	} else {
		lines = append(lines, mutedStyle.Render("Timeline spans the selected time zone's local calendar day."))
	}
	if m.status != "" {
		lines = append(lines, "", mutedStyle.Render(truncate(m.status, contentWidth)))
	}
	lines = append(lines, "", mutedStyle.Render("up/down select | ←/→ mode | c change time zone | esc back"))
	return strings.Join(lines, "\n")
}

func normalizeDaylightTimezone(timezone string) string {
	timezone, _ = normalizeDaylightReference(timezone, "UTC")
	return timezone
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
	utcState := "off"
	if m.showUTC {
		utcState = "on"
	}
	lines = append(lines, mutedStyle.Render(truncate("UTC reference: "+utcState, contentWidth)))
	lines = append(lines, mutedStyle.Render(truncate("up/down select | o sort | u toggle UTC reference | a add | d remove | esc back", contentWidth)))
	lines = append(lines, mutedStyle.Render("c columns | f formats"))
	return strings.Join(lines, "\n")
}

func (m model) viewColumns() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	contentWidth := max(1, width-2)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("24"))
	lines := []string{
		titleStyle.Render("Manage columns"),
		mutedStyle.Render(strings.Repeat("─", contentWidth)),
		"  [x] City (fixed first)",
	}
	for index, preference := range normalizeColumnPreferences(m.columns) {
		marker := "  "
		style := lipgloss.NewStyle()
		if index == m.selectedColumn {
			marker = "> "
			style = selectedStyle
		}
		visible := " "
		if preference.Visible {
			visible = "x"
		}
		line := fmt.Sprintf("%s[%s] %s", marker, visible, clockColumnFor(preference.ID).Name)
		lines = append(lines, style.Render(truncate(line, contentWidth)))
	}
	if m.status != "" {
		lines = append(lines, "", mutedStyle.Render(truncate(m.status, contentWidth)))
	}
	lines = append(lines, "", mutedStyle.Render(truncate("up/down select | space toggle | ←/→ reorder | r reset | d daylight settings | esc back", contentWidth)))
	lines = append(lines, mutedStyle.Render("m cities | f formats"))
	return strings.Join(lines, "\n")
}

func (m model) viewFormats() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	contentWidth := max(1, width-2)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("24"))
	sample := formatPreviewTime()
	lines := []string{
		titleStyle.Render("Date, time, and timezone formats"),
		mutedStyle.Render(strings.Repeat("─", contentWidth)),
	}
	for _, setting := range []formatSetting{dateFormatSetting, timeFormatSetting, timezoneFormatSetting} {
		lines = append(lines, sectionStyle.Render(formatSettingName(setting)))
		options := formatOptionsFor(setting)
		currentIndex := m.currentFormatOptionIndex(setting)
		selectedIndex := currentIndex
		if setting == m.selectedFormat {
			selectedIndex = m.selectedFormatOption
		}
		for index, option := range options {
			marker := "  "
			style := lipgloss.NewStyle()
			if setting == m.selectedFormat && index == selectedIndex {
				marker = "> "
				style = selectedStyle
			}
			checked := " "
			if index == currentIndex {
				checked = "x"
			}
			line := fmt.Sprintf("%s[%s] %-24s %s", marker, checked, option.Name, m.formatOptionExample(setting, option, index, sample))
			lines = append(lines, style.Render(truncate(line, contentWidth)))
		}
	}
	if m.formatEditing {
		lines = append(lines, "", "Custom Go time layout:", m.formatInput.View())
	}
	hint := "Date: Mon/Monday weekday, Jan/January month, 2/02 day, 2006 year."
	if m.selectedFormat == timeFormatSetting {
		hint = "Time: 15=24h, 3/03=12h, 04=minute, 05=second, PM=marker, MST=zone."
	} else if m.selectedFormat == timezoneFormatSetting {
		hint = "Abbreviations depend on the date (Berlin: CET in winter, CEST in summer)."
	}
	lines = append(lines, mutedStyle.Render(truncate(hint, contentWidth)))
	if m.formatEditing {
		lines = append(lines, mutedStyle.Render(truncate("Go reference: Mon Jan 2 15:04:05 MST 2006.", contentWidth)))
	}
	if m.status != "" {
		lines = append(lines, "", mutedStyle.Render(truncate(m.status, contentWidth)))
	}
	controls := "up/down setting | ←/→ option | enter edit Custom | esc back"
	if m.formatEditing {
		controls = "type Go layout | enter save | esc cancel"
	}
	lines = append(lines, "", mutedStyle.Render(truncate(controls, contentWidth)))
	if !m.formatEditing {
		lines = append(lines, mutedStyle.Render("m cities | c columns"))
	}
	return strings.Join(lines, "\n")
}

func formatSettingName(setting formatSetting) string {
	switch setting {
	case dateFormatSetting:
		return "Date format"
	case timeFormatSetting:
		return "Time format"
	default:
		return "Timezone name"
	}
}

func (m model) formatOptionExample(setting formatSetting, option formatOption, index int, sample time.Time) string {
	switch setting {
	case dateFormatSetting, timeFormatSetting:
		layout := option.Layout
		if layout == "" {
			if m.formatEditing && m.selectedFormat == setting {
				layout = m.formatInput.Value()
			} else if m.currentFormatOptionIndex(setting) == index {
				layout = m.currentFormatLayout(setting)
			}
			if layout == "" {
				return "Enter a custom layout"
			}
		}
		return sample.Format(layout)
	default:
		if index == 0 {
			return timezoneNameValue("Europe/Berlin", sample, timezoneNameIANA)
		}
		berlin, _ := time.LoadLocation("Europe/Berlin")
		winterName, _ := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC).In(berlin).Zone()
		summerName, _ := time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC).In(berlin).Zone()
		return "Berlin: " + winterName + " / " + summerName
	}
}
