package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

func loadWatchlist(configPath, legacyPath string, defaults []city) ([]city, error) {
	cities, err := readCities(configPath)
	if err == nil {
		return cities, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load user city config: %w", err)
	}

	cities, err = readCities(legacyPath)
	if err == nil {
		if err := saveCities(configPath, cities); err != nil {
			return nil, fmt.Errorf("migrate legacy city config: %w", err)
		}
		return cities, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load legacy city config: %w", err)
	}

	return append([]city(nil), defaults...), nil
}

func readCities(filename string) ([]city, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var cities []city
	if err := json.Unmarshal(contents, &cities); err != nil {
		return nil, err
	}
	if cities == nil {
		return nil, fmt.Errorf("city config %q must contain a JSON array", filename)
	}
	return cities, nil
}

func saveCities(filename string, cities []city) error {
	directory := filepath.Dir(filename)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(cities, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".cities-*.json")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return err
	}
	return nil
}

type citySaveResultMsg struct {
	err error
}

func saveCitiesCmd(filename string, cities []city) tea.Cmd {
	snapshot := append([]city(nil), cities...)
	return func() tea.Msg {
		return citySaveResultMsg{err: saveCities(filename, snapshot)}
	}
}
