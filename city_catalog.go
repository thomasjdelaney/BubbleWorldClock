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

func (c cityRecord) Title() string {
	return c.Name
}

func (c cityRecord) Description() string {
	parts := []string{c.Region, c.Country, c.Timezone}
	return strings.Join(nonEmpty(parts), " | ")
}

func (c cityRecord) FilterValue() string {
	return strings.Join(nonEmpty([]string{
		c.Name,
		c.ASCIIName,
		c.Region,
		c.Admin1Code,
		c.Country,
		c.CountryCode,
		c.Timezone,
	}), "\t")
}

func cityFilter(term string, targets []string) []list.Rank {
	cityNames := make([]string, len(targets))
	otherFields := make([]string, len(targets))
	for i, target := range targets {
		fields := strings.SplitN(target, "\t", 3)
		cityNames[i] = strings.Join(fields[:min(2, len(fields))], " ")
		if len(fields) > 2 {
			otherFields[i] = fields[2]
		}
	}
	if matches := list.DefaultFilter(term, cityNames); len(matches) > 0 {
		return matches
	}
	return list.DefaultFilter(term, otherFields)
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
