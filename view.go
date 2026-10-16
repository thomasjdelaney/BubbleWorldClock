package main

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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
	case columnsScreen:
		content = m.viewColumns()
	case formatScreen:
		content = m.viewFormats()
	case daylightScreen:
		content = m.viewDaylightSettings()
	case pickerScreen:
		content = m.picker.View()
		if m.status != "" {
			content += "\n\n" + m.status
		}
		pickerAction := "enter add"
		if m.pickerPurpose == daylightTimezonePicker {
			pickerAction = "enter choose time zone"
		}
		content += "\n\n" + truncate("country:Japan | tz:Paris | "+pickerAction+" | esc back", max(1, m.width))
	default:
		content = m.viewClock()
	}
	if helpView := m.help.View(screenHelpKeyMap{keyMap: m.keys, screen: m.screen}); helpView != "" && m.screen != pickerScreen && !m.formatEditing {
		content += "\n\n" + helpView
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m model) viewClock() string {
	const (
		dayFormat    = "Mon"
		offsetFormat = "-07:00"
	)

	width := m.width
	if width == 0 {
		width = 80
	}
	contentWidth := max(1, width-2)
	columns := visibleClockColumns(m.columns)
	for index := range columns {
		switch columns[index].ID {
		case columnDate:
			columns[index].Width = clockValueColumnWidth(m.cities, m.now, normalizeDateLayout(m.dateFormat), "DATE", m.showUTC)
		case columnTime:
			columns[index].Width = clockValueColumnWidth(m.cities, m.now, normalizeTimeLayout(m.timeFormat), "TIME", m.showUTC)
		case columnTimezone:
			columns[index].Width = timezoneColumnWidth(m.cities, m.now, m.timezoneNameStyle, m.showUTC)
		}
	}

	desiredCityWidth := 4
	for i := range m.cities {
		desiredCityWidth = max(desiredCityWidth, lipgloss.Width(m.cityLabel(i)))
	}
	widths, gap, compact := clockLayoutWidths(contentWidth, desiredCityWidth, columns)
	tableWidth := gap * (len(widths) - 1)
	for _, width := range widths {
		tableWidth += width
	}
	cityWidth := widths[0]
	barWidth := 0
	for index, column := range columns {
		if column.ID == columnDaylight {
			barWidth = widths[index+1]
		}
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
	cityStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	barStyles := map[lightLevel]lipgloss.Style{
		lightNight:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		lightTwilight: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		lightDay:      lipgloss.NewStyle().Foreground(lipgloss.Color("229")),
	}
	markerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	utcStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	daylightLocation, err := time.LoadLocation(m.daylightTimezone)
	if err != nil {
		daylightLocation = time.UTC
	}
	daylightMode := normalizeDaylightMode(m.daylightMode)
	daylightStart, daylightEnd := daylightWindowForMode(m.now, daylightLocation, daylightMode)
	lines := []string{titleStyle.Render(truncate("Bubble World Clock", contentWidth))}
	if !compact {
		lines = append(lines, mutedStyle.Render(strings.Repeat("─", tableWidth)))
	}
	header := []string{padRight(truncate("CITY", cityWidth), cityWidth)}
	for index, column := range columns {
		header = append(header, padRight(truncate(column.Label, widths[index+1]), widths[index+1]))
	}
	lines = append(lines, headerStyle.Render(strings.Join(header, strings.Repeat(" ", gap))))
	if !compact {
		lines = append(lines, borderStyle.Render(strings.Repeat("─", tableWidth)))
	}
	if len(m.cities) == 0 {
		lines = append(lines, mutedStyle.Render("No cities yet. Press m for cities, c for columns, or f for formats."))
	}

	utcIndex := len(m.cities)
	if m.showUTC {
		utc := city{Name: "UTC", Timezone: "UTC"}
		for index, city := range m.cities {
			if cityComesBefore(utc, city, m.sortMode, m.now) {
				utcIndex = index
				break
			}
		}
	}
	appendClockRow := func(name string, nameStyle lipgloss.Style, values map[columnID]string, daylightBar string, rowIndex int) {
		cells := []string{withRowBackground(nameStyle, rowIndex).Render(padRight(truncate(name, cityWidth), cityWidth))}
		for index, column := range columns {
			cellWidth := widths[index+1]
			if column.ID == columnDaylight && daylightBar != "" {
				cells = append(cells, daylightBar)
				continue
			}
			value := padRight(truncate(values[column.ID], cellWidth), cellWidth)
			cells = append(cells, withRowBackground(timeStyle, rowIndex).Render(value))
		}
		var row strings.Builder
		for index, cell := range cells {
			if index > 0 {
				row.WriteString(withRowBackground(lipgloss.NewStyle(), rowIndex).Render(strings.Repeat(" ", gap)))
			}
			row.WriteString(cell)
		}
		lines = append(lines, row.String())
	}
	appendUTCRow := func(rowIndex int) {
		utcTime := m.now.UTC()
		values := map[columnID]string{
			columnDay:       utcTime.Format(dayFormat),
			columnDate:      utcTime.Format(normalizeDateLayout(m.dateFormat)),
			columnTime:      utcTime.Format(normalizeTimeLayout(m.timeFormat)),
			columnUTCOffset: utcTime.Format(offsetFormat),
			columnTimezone:  timezoneNameValue("UTC", utcTime, m.timezoneNameStyle),
		}
		appendClockRow("UTC", utcStyle, values, "", rowIndex)
	}

	for index, city := range m.cities {
		if m.showUTC && index == utcIndex {
			appendUTCRow(index)
		}
		rowIndex := index
		if m.showUTC && index >= utcIndex {
			rowIndex++
		}
		location, err := time.LoadLocation(city.Timezone)
		if err != nil {
			values := map[columnID]string{
				columnTime:     "invalid",
				columnTimezone: "invalid",
				columnDaylight: "invalid time zone",
			}
			nameStyle := cityStyle
			if compact {
				nameStyle = timeStyle
			}
			appendClockRow(m.cityLabel(index), nameStyle, values, "", rowIndex)
			continue
		}

		localTime := m.now.In(location)
		values := map[columnID]string{
			columnDay:       localTime.Format(dayFormat),
			columnDate:      localTime.Format(normalizeDateLayout(m.dateFormat)),
			columnTime:      localTime.Format(normalizeTimeLayout(m.timeFormat)),
			columnUTCOffset: localTime.Format(offsetFormat),
			columnTimezone:  timezoneNameValue(city.Timezone, localTime, m.timezoneNameStyle),
		}
		daylightBar := ""
		if barWidth > 0 {
			intervals, hasCoordinates := cityDaylightIntervalsInWindow(city, m.catalog, location, daylightStart, daylightEnd)
			daylightBar = renderDaylightBarForMode(intervals, hasCoordinates, barWidth, daylightStart, daylightEnd, m.now, daylightMode, rowIndex, barStyles, markerStyle, mutedStyle)
		}
		nameStyle := cityStyle
		if compact {
			nameStyle = timeStyle
		}
		appendClockRow(m.cityLabel(index), nameStyle, values, daylightBar, rowIndex)
	}
	if m.showUTC && utcIndex == len(m.cities) {
		appendUTCRow(utcIndex)
	}
	if barWidth > 0 {
		legend := barStyles[lightNight].Render(nightDotGlyph+nightStarGlyph) + " night  " + barStyles[lightTwilight].Render(twilightGlyph) + " twilight  " + barStyles[lightDay].Render(daylightGlyph) + " daylight  " + markerStyle.Render(nowGlyph) + " now (" + m.daylightLegendCityName() + ")"
		lines = append(lines, mutedStyle.Render(truncate(legend, contentWidth)))
	}
	lines = append(lines, mutedStyle.Render(truncate("m manage cities | c manage columns | f manage formats", contentWidth)))
	return strings.Join(lines, "\n")
}

func (m model) daylightLegendCityName() string {
	_, cityName := normalizeDaylightReference(m.daylightTimezone, m.daylightCityName)
	return cityName
}

func timezoneNameValue(timezone string, localTime time.Time, style timezoneNameStyle) string {
	value := timezone
	if style == timezoneNameAbbreviation {
		name, _ := localTime.Zone()
		value = name
	}
	return strings.ReplaceAll(value, "_", " ")
}

func timezoneColumnWidth(cities []city, now time.Time, style timezoneNameStyle, showUTC bool) int {
	width := lipgloss.Width(clockColumnFor(columnTimezone).Label)
	if showUTC {
		width = max(width, lipgloss.Width(timezoneNameValue("UTC", now.UTC(), style)))
	}
	for _, value := range cities {
		location, err := time.LoadLocation(value.Timezone)
		if err != nil {
			width = max(width, lipgloss.Width("invalid"))
			continue
		}
		width = max(width, lipgloss.Width(timezoneNameValue(value.Timezone, now.In(location), style)))
	}
	return width
}

func clockValueColumnWidth(cities []city, now time.Time, layout, label string, showUTC bool) int {
	width := lipgloss.Width(label)
	if showUTC {
		width = max(width, lipgloss.Width(now.UTC().Format(layout)))
	}
	for _, value := range cities {
		location, err := time.LoadLocation(value.Timezone)
		if err != nil {
			width = max(width, lipgloss.Width("invalid"))
			continue
		}
		width = max(width, lipgloss.Width(now.In(location).Format(layout)))
	}
	return width
}

func (m model) cityLabel(index int) string {
	if index < 0 || index >= len(m.cities) {
		return ""
	}
	return m.cityLabelFor(m.cities[index])
}

func (m model) cityLabelFor(city city) string {
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
