package main

import (
	"strings"

	"charm.land/bubbles/v2/list"
)

type cityRecord struct {
	GeoNameID   int64  `json:"geoname_id"`
	Name        string `json:"name"`
	ASCIIName   string `json:"ascii_name"`
	CountryCode string `json:"country_code"`
	Country     string `json:"country"`
	Admin1Code  string `json:"admin1_code"`
	Region      string `json:"region"`
	Timezone    string `json:"timezone"`
	Population  int64  `json:"population"`
}

const (
	cityNameField = iota
	cityASCIINameField
	cityRegionField
	cityAdmin1CodeField
	cityCountryField
	cityCountryCodeField
	cityTimezoneField
	cityFilterFieldCount
)

func (c cityRecord) Title() string {
	return c.Name
}

func (c cityRecord) Description() string {
	parts := []string{c.Region, c.Country, c.Timezone}
	return strings.Join(nonEmpty(parts), " | ")
}

func (c cityRecord) FilterValue() string {
	return strings.Join([]string{
		c.Name,
		c.ASCIIName,
		c.Region,
		c.Admin1Code,
		c.Country,
		c.CountryCode,
		c.Timezone,
	}, "\t")
}

func cityFilter(term string, targets []string) []list.Rank {
	cityNames := make([]string, len(targets))
	otherFields := make([]string, len(targets))
	countries := make([]string, len(targets))
	timezones := make([]string, len(targets))
	for i, target := range targets {
		fields := strings.SplitN(target, "\t", cityFilterFieldCount)
		for len(fields) < cityFilterFieldCount {
			fields = append(fields, "")
		}
		cityNames[i] = strings.Join(nonEmpty(fields[cityNameField:cityASCIINameField+1]), " ")
		otherFields[i] = strings.Join(nonEmpty(fields[cityRegionField:cityFilterFieldCount]), " ")
		countries[i] = strings.Join(nonEmpty(fields[cityCountryField:cityCountryCodeField+1]), " ")
		timezones[i] = fields[cityTimezoneField]
	}

	if prefix, value, hasPrefix := strings.Cut(term, ":"); hasPrefix {
		switch strings.ToLower(strings.TrimSpace(prefix)) {
		case "country":
			return filterCityField(strings.TrimSpace(value), countries)
		case "tz":
			return filterCityField(strings.TrimSpace(value), timezones)
		}
	}
	if matches := list.DefaultFilter(term, cityNames); len(matches) > 0 {
		return matches
	}
	return list.DefaultFilter(term, otherFields)
}

func filterCityField(term string, fields []string) []list.Rank {
	if term == "" {
		matches := make([]list.Rank, len(fields))
		for index := range fields {
			matches[index].Index = index
		}
		return matches
	}
	return list.DefaultFilter(term, fields)
}

func nonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

var _ list.DefaultItem = cityRecord{}
