package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func (m model) openPicker() (tea.Model, tea.Cmd) {
	return m.openCityPicker(addCityPicker, manageScreen)
}

func (m model) openDaylightPicker() (tea.Model, tea.Cmd) {
	return m.openCityPicker(daylightTimezonePicker, daylightScreen)
}

func (m model) openCityPicker(purpose pickerPurpose, returnScreen screen) (tea.Model, tea.Cmd) {
	items := make([]list.Item, len(m.catalog))
	for i, record := range m.catalog {
		items[i] = record
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	m.picker = list.New(items, delegate, max(1, m.width), max(1, m.height-6))
	m.picker.Filter = cityFilter
	if purpose == daylightTimezonePicker {
		m.picker.Title = "Choose a city time zone"
	} else {
		m.picker.Title = "Add a city"
	}
	m.picker.SetShowHelp(false)
	m.picker.SetStatusBarItemName("city", "cities")
	m.picker.DisableQuitKeybindings()
	m.picker.SetFilterText("")
	m.picker.SetFilterState(list.Filtering)
	m.pickerPurpose = purpose
	m.pickerReturnScreen = returnScreen
	m.screen = pickerScreen
	m.status = ""
	return m, textinput.Blink
}

func (m model) addCity(record cityRecord) (tea.Model, tea.Cmd) {
	if m.saving {
		m.status = "Wait for the current save to finish"
		return m, nil
	}
	if _, err := time.LoadLocation(record.Timezone); err != nil {
		m.status = "Catalog timezone is invalid: " + record.Timezone
		return m, nil
	}
	newCity := city{
		GeoNameID:   record.GeoNameID,
		Name:        record.Name,
		CountryCode: record.CountryCode,
		Country:     record.Country,
		Admin1Code:  record.Admin1Code,
		Region:      record.Region,
		Timezone:    record.Timezone,
	}
	for _, existing := range m.cities {
		if sameCity(existing, newCity) {
			m.screen = manageScreen
			m.status = "That city is already on the clock"
			return m, nil
		}
	}
	m.cities = append(m.cities, newCity)
	m.selected = len(m.cities) - 1
	m.sortCitiesPreservingSelection()
	m.screen = manageScreen
	return m, m.startSave()
}

func cityRecordIdentity(value cityRecord) string {
	if value.GeoNameID != 0 {
		return fmt.Sprintf("id:%d", value.GeoNameID)
	}
	return "city:" + strings.ToLower(value.Name) + "\x00" + strings.ToLower(value.Country) + "\x00" + value.Timezone
}

func sameCity(first, second city) bool {
	if first.GeoNameID != 0 && second.GeoNameID != 0 {
		return first.GeoNameID == second.GeoNameID
	}
	return strings.EqualFold(first.Name, second.Name) && first.Timezone == second.Timezone
}

func (m model) removeCity() (tea.Model, tea.Cmd) {
	if m.saving {
		m.status = "Wait for the current save to finish"
		return m, nil
	}
	if len(m.cities) == 0 {
		m.status = "No cities to remove"
		return m, nil
	}
	m.cities = append(m.cities[:m.selected], m.cities[m.selected+1:]...)
	if len(m.cities) > 0 {
		m.selected = min(m.selected, len(m.cities)-1)
	} else {
		m.selected = 0
	}
	if m.selected >= len(m.cities) {
		m.selected = max(0, len(m.cities)-1)
	}
	return m, m.startSave()
}

func cityIdentity(value city) string {
	if value.GeoNameID != 0 {
		return fmt.Sprintf("id:%d", value.GeoNameID)
	}
	return "city:" + strings.ToLower(value.Name) + "\x00" + value.Timezone
}

func (m *model) sortCitiesPreservingSelection() {
	if !m.sortMode.valid() {
		m.sortMode = sortNameAscending
	}
	var selectedIdentity string
	if m.selected >= 0 && m.selected < len(m.cities) {
		selectedIdentity = cityIdentity(m.cities[m.selected])
	}
	sort.SliceStable(m.cities, func(first, second int) bool {
		return cityComesBefore(m.cities[first], m.cities[second], m.sortMode, m.now)
	})
	if selectedIdentity == "" {
		m.selected = min(m.selected, max(0, len(m.cities)-1))
		return
	}
	for index, value := range m.cities {
		if cityIdentity(value) == selectedIdentity {
			m.selected = index
			return
		}
	}
	m.selected = min(m.selected, max(0, len(m.cities)-1))
}

func cityComesBefore(left, right city, mode sortMode, now time.Time) bool {
	if !mode.valid() {
		mode = sortNameAscending
	}
	leftLocation, leftErr := time.LoadLocation(left.Timezone)
	rightLocation, rightErr := time.LoadLocation(right.Timezone)
	leftOffset, rightOffset := 0, 0
	if leftErr == nil {
		_, leftOffset = now.In(leftLocation).Zone()
	}
	if rightErr == nil {
		_, rightOffset = now.In(rightLocation).Zone()
	}
	if mode == sortOffsetAscending || mode == sortOffsetDescending {
		if leftErr != rightErr {
			return leftErr == nil
		}
		if leftOffset != rightOffset {
			if mode == sortOffsetDescending {
				return leftOffset > rightOffset
			}
			return leftOffset < rightOffset
		}
	}
	leftName := strings.ToLower(left.Name)
	rightName := strings.ToLower(right.Name)
	if leftName != rightName {
		if mode == sortNameDescending {
			return leftName > rightName
		}
		return leftName < rightName
	}
	if left.Timezone != right.Timezone {
		return left.Timezone < right.Timezone
	}
	return left.GeoNameID < right.GeoNameID
}
