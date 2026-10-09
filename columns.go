package main

type columnID string

const (
	columnDay       columnID = "day"
	columnDate      columnID = "date"
	columnTime      columnID = "time"
	columnUTCOffset columnID = "utc_offset"
	columnDaylight  columnID = "daylight"
)

type columnPreference struct {
	ID      columnID `json:"id"`
	Visible bool     `json:"visible"`
}

type clockColumn struct {
	ID       columnID
	Label    string
	Name     string
	Width    int
	Flexible bool
	MaxWidth int
}

func defaultColumnPreferences() []columnPreference {
	return []columnPreference{
		{ID: columnDay, Visible: true},
		{ID: columnDate, Visible: true},
		{ID: columnTime, Visible: true},
		{ID: columnUTCOffset, Visible: true},
		{ID: columnDaylight, Visible: true},
	}
}

func normalizeColumnPreferences(preferences []columnPreference) []columnPreference {
	defaults := defaultColumnPreferences()
	if len(preferences) == 0 {
		return defaults
	}

	known := make(map[columnID]bool, len(defaults))
	for _, preference := range defaults {
		known[preference.ID] = true
	}
	seen := make(map[columnID]bool, len(defaults))
	result := make([]columnPreference, 0, len(defaults))
	for _, preference := range preferences {
		if !known[preference.ID] || seen[preference.ID] {
			continue
		}
		seen[preference.ID] = true
		result = append(result, preference)
	}
	for _, preference := range defaults {
		if !seen[preference.ID] {
			result = append(result, preference)
		}
	}
	return result
}

func visibleClockColumns(preferences []columnPreference) []clockColumn {
	columns := make([]clockColumn, 0, len(preferences))
	for _, preference := range normalizeColumnPreferences(preferences) {
		if preference.Visible {
			columns = append(columns, clockColumnFor(preference.ID))
		}
	}
	return columns
}

func clockColumnFor(id columnID) clockColumn {
	switch id {
	case columnDay:
		return clockColumn{ID: id, Label: "DAY", Name: "Day", Width: 3}
	case columnDate:
		return clockColumn{ID: id, Label: "DATE", Name: "Date", Width: len("02 Jan 2006")}
	case columnTime:
		return clockColumn{ID: id, Label: "TIME", Name: "Time", Width: len("15:04:05")}
	case columnUTCOffset:
		return clockColumn{ID: id, Label: "UTC OFFSET", Name: "UTC offset", Width: len("UTC OFFSET")}
	case columnDaylight:
		return clockColumn{ID: id, Label: "DAYLIGHT", Name: "Daylight", Width: len("DAYLIGHT"), Flexible: true, MaxWidth: 48}
	default:
		return clockColumn{ID: id, Label: string(id), Name: string(id), Width: 1}
	}
}

func clockLayoutWidths(contentWidth, cityWidth int, columns []clockColumn) ([]int, int, bool) {
	widths := make([]int, len(columns)+1)
	widths[0] = cityWidth
	for index, column := range columns {
		widths[index+1] = column.Width
	}

	gap := 2
	totalWidth := func() int {
		total := gap * (len(widths) - 1)
		for _, width := range widths {
			total += width
		}
		return total
	}
	compact := totalWidth() > contentWidth
	if compact {
		gap = 1
		if contentWidth < len(widths)*2-1 {
			gap = 0
		}
		for totalWidth() > contentWidth {
			largest := -1
			for index, width := range widths {
				if width > 1 && (largest < 0 || width > widths[largest]) {
					largest = index
				}
			}
			if largest < 0 {
				break
			}
			widths[largest]--
		}
	} else {
		remaining := contentWidth - totalWidth()
		for index, column := range columns {
			if !column.Flexible || remaining == 0 {
				continue
			}
			extra := min(remaining, column.MaxWidth-widths[index+1])
			widths[index+1] += extra
			remaining -= extra
		}
		widths[0] += remaining
	}
	return widths, gap, compact
}

func moveColumnPreference(preferences []columnPreference, index, direction int) ([]columnPreference, int) {
	preferences = append([]columnPreference(nil), preferences...)
	target := index + direction
	if index < 0 || index >= len(preferences) || target < 0 || target >= len(preferences) {
		return preferences, index
	}
	preferences[index], preferences[target] = preferences[target], preferences[index]
	return preferences, target
}
