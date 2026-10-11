package main

import (
	"fmt"
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
	case pickerScreen:
		content = m.picker.View()
		if m.status != "" {
			content += "\n\n" + m.status
		}
		content += "\n\n" + truncate("country:Japan | tz:Paris | enter add | esc back", max(1, m.width))
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

	utcDayStart := time.Date(m.now.UTC().Year(), m.now.UTC().Month(), m.now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	utcDayEnd := utcDayStart.Add(24 * time.Hour)
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
			intervals, hasCoordinates := cityDaylightIntervals(city, m.catalog, location, utcDayStart)
			daylightBar = renderDaylightBar(intervals, hasCoordinates, barWidth, utcDayStart, utcDayEnd, m.now, rowIndex, barStyles, markerStyle, mutedStyle)
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
		legend := barStyles[lightNight].Render(nightDotGlyph+nightStarGlyph) + " night  " + barStyles[lightTwilight].Render(twilightGlyph) + " twilight  " + barStyles[lightDay].Render(daylightGlyph) + " daylight  " + markerStyle.Render(nowGlyph) + " now"
		lines = append(lines, mutedStyle.Render(truncate(legend, contentWidth)))
	}
	lines = append(lines, mutedStyle.Render(truncate("m manage cities | c manage columns | f manage formats", contentWidth)))
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
	lines = append(lines, "", mutedStyle.Render(truncate("up/down select | space toggle | ←/→ reorder | r reset | esc back", contentWidth)))
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
