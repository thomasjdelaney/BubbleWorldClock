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
	case pickerScreen:
		content = m.picker.View()
		if m.status != "" {
			content += "\n\n" + m.status
		}
		content += "\n\n" + truncate("country:Japan | tz:Paris | enter add | esc back", max(1, m.width))
	default:
		content = m.viewClock()
	}
	if helpView := m.help.View(screenHelpKeyMap{keyMap: m.keys, screen: m.screen}); helpView != "" && m.screen != pickerScreen {
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
		shortTime    = "15:04"
		offsetFormat = "-07:00"
	)

	width := m.width
	if width == 0 {
		width = 80
	}
	contentWidth := max(1, width-2)
	columns := visibleClockColumns(m.columns)

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
		lines = append(lines, mutedStyle.Render("No cities yet. Press m for cities or c for columns."))
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
		timeLayout := timeFormat
		if compact {
			timeLayout = shortTime
		}
		values := map[columnID]string{
			columnDay:       utcTime.Format(dayFormat),
			columnDate:      utcTime.Format(dateFormat),
			columnTime:      utcTime.Format(timeLayout),
			columnUTCOffset: utcTime.Format(offsetFormat),
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
		timeLayout := timeFormat
		if compact {
			timeLayout = shortTime
		}
		values := map[columnID]string{
			columnDay:       localTime.Format(dayFormat),
			columnDate:      localTime.Format(dateFormat),
			columnTime:      localTime.Format(timeLayout),
			columnUTCOffset: localTime.Format(offsetFormat),
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
	lines = append(lines, mutedStyle.Render(truncate("m manage cities | c manage columns", contentWidth)))
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
	lines = append(lines, mutedStyle.Render(truncate("up/down select | o sort | u toggle UTC reference | c columns | a add | d remove | esc back", contentWidth)))
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
	return strings.Join(lines, "\n")
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
