package main

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	sunrise "github.com/nathan-osman/go-sunrise"
)

type lightLevel uint8

const (
	lightNight lightLevel = iota
	lightTwilight
	lightDay
)

const (
	nightDotGlyph  = "·"
	nightStarGlyph = "*"
	twilightGlyph  = "▒"
	daylightGlyph  = "━"
	nowGlyph       = "│"
)

type lightInterval struct {
	start time.Time
	end   time.Time
	level lightLevel
}

func daylightWindow(now time.Time, location *time.Location) (time.Time, time.Time) {
	if location == nil {
		location = time.UTC
	}
	localNow := now.In(location)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	return start, start.AddDate(0, 0, 1)
}

func cityCoordinates(value city, catalog []cityRecord) (float64, float64, bool) {
	for _, record := range catalog {
		if value.GeoNameID != 0 && record.GeoNameID == value.GeoNameID {
			if record.CoordinatesKnown {
				return record.Latitude, record.Longitude, true
			}
			return 0, 0, false
		}
	}
	for _, record := range catalog {
		if strings.EqualFold(value.Name, record.Name) && value.Timezone == record.Timezone {
			if record.CoordinatesKnown {
				return record.Latitude, record.Longitude, true
			}
			return 0, 0, false
		}
	}
	name := normalizedCityName(value.Name)
	for _, record := range catalog {
		if name == normalizedCityName(record.Name) && value.Timezone == record.Timezone {
			if record.CoordinatesKnown {
				return record.Latitude, record.Longitude, true
			}
			return 0, 0, false
		}
	}
	return 0, 0, false
}

func normalizedCityName(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	return strings.TrimSuffix(name, " city")
}

func padRight(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func cityDaylightIntervals(value city, catalog []cityRecord, location *time.Location, utcDayStart time.Time) ([]lightInterval, bool) {
	return cityDaylightIntervalsInWindow(value, catalog, location, utcDayStart, utcDayStart.Add(24*time.Hour))
}

func cityDaylightIntervalsInWindow(value city, catalog []cityRecord, location *time.Location, windowStart, windowEnd time.Time) ([]lightInterval, bool) {
	latitude, longitude, ok := cityCoordinates(value, catalog)
	if !ok {
		return nil, false
	}
	if !windowEnd.After(windowStart) {
		return nil, true
	}

	// Cover every city-local date intersecting the selected reference day's instant window.
	localDate := windowStart.In(location)
	year, month, day := localDate.Date()
	baseDate := time.Date(year, month, day, 12, 0, 0, 0, location)
	localEnd := windowEnd.Add(-time.Nanosecond).In(location)
	endYear, endMonth, endDay := localEnd.Date()
	endDate := time.Date(endYear, endMonth, endDay, 12, 0, 0, 0, location)
	intervals := make([]lightInterval, 0, 8)
	for date := baseDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
		year, month, day := date.Date()
		rise, set := sunrise.SunriseSunset(latitude, longitude, year, month, day)
		civilRise, civilSet := sunrise.TimeOfElevation(latitude, longitude, -6, year, month, day)

		if rise.IsZero() || set.IsZero() {
			if !civilRise.IsZero() && !civilSet.IsZero() {
				intervals = append(intervals, lightInterval{civilRise, civilSet, lightTwilight})
				continue
			}
			level := lightNight
			if sunrise.Elevation(latitude, longitude, time.Date(year, month, day, 12, 0, 0, 0, location)) >= 0 {
				level = lightDay
			}
			localStart := time.Date(year, month, day, 0, 0, 0, 0, location)
			localEnd := localStart.AddDate(0, 0, 1)
			intervals = append(intervals, lightInterval{localStart.UTC(), localEnd.UTC(), level})
			continue
		}

		if !civilRise.IsZero() && civilRise.Before(rise) {
			intervals = append(intervals, lightInterval{civilRise, rise, lightTwilight})
		}
		intervals = append(intervals, lightInterval{rise, set, lightDay})
		if !civilSet.IsZero() && civilSet.After(set) {
			intervals = append(intervals, lightInterval{set, civilSet, lightTwilight})
		}
	}

	clipped := intervals[:0]
	for _, interval := range intervals {
		if interval.start.Before(windowStart) {
			interval.start = windowStart
		}
		if interval.end.After(windowEnd) {
			interval.end = windowEnd
		}
		if interval.start.Before(interval.end) {
			clipped = append(clipped, interval)
		}
	}
	return clipped, true
}

func lightAt(at time.Time, intervals []lightInterval) lightLevel {
	level := lightNight
	for _, interval := range intervals {
		if !at.Before(interval.start) && at.Before(interval.end) && interval.level > level {
			level = interval.level
		}
	}
	return level
}

func renderDaylightBar(intervals []lightInterval, hasCoordinates bool, width int, utcDayStart, utcDayEnd, now time.Time, rowIndex int, styles map[lightLevel]lipgloss.Style, markerStyle, mutedStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if !hasCoordinates {
		return withRowBackground(mutedStyle, rowIndex).Render(padRight(truncate("no coordinates", width), width))
	}

	markerIndex := daylightMarkerIndex(now, utcDayStart, utcDayEnd, width)
	var bar strings.Builder
	for index := range width {
		at := utcDayStart.Add(time.Duration((float64(index) + 0.5) / float64(width) * float64(utcDayEnd.Sub(utcDayStart))))
		if index == markerIndex {
			bar.WriteString(withRowBackground(markerStyle, rowIndex).Render(nowGlyph))
			continue
		}

		level := lightAt(at, intervals)
		glyph := nightDotGlyph
		if level == lightNight && (index+rowIndex)%5 == 0 {
			glyph = nightStarGlyph
		}
		if level == lightTwilight {
			glyph = twilightGlyph
		} else if level == lightDay {
			glyph = daylightGlyph
		}
		bar.WriteString(withRowBackground(styles[level], rowIndex).Render(glyph))
	}
	return bar.String()
}

func daylightMarkerIndex(now, utcDayStart, utcDayEnd time.Time, width int) int {
	if width <= 1 || !utcDayEnd.After(utcDayStart) {
		return 0
	}
	index := int(now.UTC().Sub(utcDayStart).Seconds() / utcDayEnd.Sub(utcDayStart).Seconds() * float64(width))
	return min(width-1, max(0, index))
}
