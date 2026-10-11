package main

import "time"

const (
	defaultDateLayout = "02 Jan 2006"
	defaultTimeLayout = "15:04:05"
)

type formatSetting uint8

const (
	dateFormatSetting formatSetting = iota
	timeFormatSetting
	timezoneFormatSetting
)

type formatOption struct {
	Name   string
	Layout string
}

var dateFormatOptions = []formatOption{
	{Name: "Day month year", Layout: defaultDateLayout},
	{Name: "ISO year-month-day", Layout: "2006-01-02"},
	{Name: "Day/month/year", Layout: "02/01/2006"},
	{Name: "Month/day/year", Layout: "01/02/2006"},
	{Name: "Custom"},
}

var timeFormatOptions = []formatOption{
	{Name: "24-hour with seconds", Layout: defaultTimeLayout},
	{Name: "24-hour", Layout: "15:04"},
	{Name: "12-hour with seconds", Layout: "3:04:05 PM"},
	{Name: "12-hour", Layout: "3:04 PM"},
	{Name: "Custom"},
}

func normalizeDateLayout(layout string) string {
	if layout == "" {
		return defaultDateLayout
	}
	return layout
}

func normalizeTimeLayout(layout string) string {
	if layout == "" {
		return defaultTimeLayout
	}
	return layout
}

func formatOptionsFor(setting formatSetting) []formatOption {
	switch setting {
	case dateFormatSetting:
		return dateFormatOptions
	case timeFormatSetting:
		return timeFormatOptions
	default:
		return []formatOption{{Name: "IANA ID"}, {Name: "Local abbreviation"}}
	}
}

func formatOptionIndex(layout string, options []formatOption) int {
	for index, option := range options {
		if option.Layout != "" && option.Layout == layout {
			return index
		}
	}
	return len(options) - 1
}

func formatPreviewTime() time.Time {
	zone := time.FixedZone("MST", -7*60*60)
	return time.Date(2006, time.January, 2, 15, 4, 5, 0, zone)
}
